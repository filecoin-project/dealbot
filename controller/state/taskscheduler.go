package state

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/google/uuid"
	dagjson "github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/robfig/cron/v3"
)

const (
	schedulerOwner = "dealbot_scheduler"
)

var noEnt cron.EntryID

// job is the scheduler's internal record of a scheduled job
type job struct {
	entryID  cron.EntryID
	expireAt time.Time
	idChan   chan cron.EntryID
	limit    string
	schedule string
	sdb      *stateDB
	runCount int
	taskID   string
}

// Run creates a copy of the task without the schedule for PopTask
func (j *job) Run() {
	if j.entryID == noEnt {
		j.entryID = <-j.idChan
	}
	j.sdb.runTask(j.taskID, j.schedule, j.limit, j.expireAt, j.runCount, j.entryID)
	j.runCount++
}

func (s *stateDB) scheduleTask(task tasks.Task) error {
	taskID := task.UUID.String()
	taskSchedule, limit := task.Schedule()
	if taskSchedule == "" {
		log.Infow("task became unscheduled, unassigning from scheduler", "taskID", taskID)
		s.unassignScheduledTask(taskID)
		return nil
	}

	log.Infow("scheduling task", "uuid", taskID, "schedule", taskSchedule, "schedule_limit", limit)

	j := &job{
		idChan:   make(chan cron.EntryID, 1),
		schedule: taskSchedule,
		sdb:      s,
		taskID:   taskID,
	}
	if limit != "" {
		duration, err := time.ParseDuration(limit)
		if err != nil {
			return fmt.Errorf("invalid schedule limit %q: %s", limit, err)
		}

		j.limit = limit
		j.expireAt = time.Now().Add(duration)
	}
	entID, err := s.cronSched.AddJob(taskSchedule, j)
	if err != nil {
		return fmt.Errorf("invalid schedule specification %q: %s", taskSchedule, err)
	}
	j.idChan <- entID

	s.logNextRunTime(taskID, entID)
	return nil
}

func (s *stateDB) runTask(taskID, schedule, limit string, expireAt time.Time, runCount int, jobID cron.EntryID) {
	task, tag, err := s.getWithTag(context.Background(), taskID)
	if err != nil {
		log.Errorw("cannot load scheduled task from database", "taskID", taskID, "err", err)
		return
	}

	// If task removed then stop scheduling it
	if task == nil {
		log.Infow("scheduled task removed", "taskID", taskID)
		s.cronSched.Remove(jobID)
		return
	}

	// If task's schedule changed, rescheduled it
	taskSchedule, schedLimit := task.Schedule()
	if taskSchedule != schedule || schedLimit != limit {
		log.Infow("task scheduled changed, rescheduling task", "taskID", taskID)
		s.cronSched.Remove(jobID)
		s.scheduleTask(task)
		return
	}

	ctx := context.Background()

	// If the schedule has expired, remove job from scheduler and set ownership
	// to the expired task owner.
	if !expireAt.IsZero() && time.Now().After(expireAt) {
		log.Infow("scheduling expired for task", "taskID", taskID)
		s.cronSched.Remove(jobID)
		err = s.transact(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, deleteTaskSQL, taskID)
			return err
		})
		if err != nil {
			log.Errorw("could not assign task to expired task owner", "taskID", taskID, "err", err)
		}
		return
	}

	// Check for a previous generated task that is not yet finished
	var count int
	err = s.transact(ctx, func(tx *sql.Tx) error {
		return s.db().QueryRowContext(ctx, countChildTasksLTStatusSQL, taskID, tasks.Successful.Int()).Scan(&count)
	})
	if err != nil {
		log.Errorw("could not check for unfinished previous task", "err", err)
	}
	if count != 0 {
		log.Warnw("skipping scheduled task generation, previous task incomplete", "parentTaskID", taskID)
		return
	}

	// Generate a new runable task
	newTaskID, err := s.createRunableTask(task, tag, taskID, runCount)
	if err != nil {
		log.Errorw("cannot create runable task", "scheduledTaskID", taskID, "err", err)
		return
	}

	log.Infow("created new runable task from scheduled task", "runableTaskID", newTaskID, "scheduledTaskID", taskID)
	s.logNextRunTime(taskID, jobID)

	if s.runNotice != nil {
		select {
		case s.runNotice <- newTaskID:
		default:
		}
	}
}

// createRunableTask generates a new runable (not scheduled) task from a scheduled task
func (s *stateDB) createRunableTask(task tasks.Task, tag, parent string, runCount int) (string, error) {
	newTaskID := uuid.New().String()
	runableTask := task.MakeRunable(newTaskID, runCount)
	err := s.saveTask(context.Background(), runableTask, tag, parent)
	if err != nil {
		return "", err
	}
	return newTaskID, nil
}

// unassignScheduledTask clears task ownership, in DB table only
func (s *stateDB) unassignScheduledTask(taskID string) error {
	ctx := context.Background()
	err := s.transact(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, unassignScheduledTaskSQL, taskID)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}

// recoverScheduledTasks reads all tasks that are assigned to the scheduler and
// schedules them.  This should only be called during controller startup to
// recover tasks that were scheduled during a previous run of the controller.
func (s *stateDB) recoverScheduledTasks(ctx context.Context) (int, error) {
	var tasklist []tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, getAllTasksForOwnerSQL, schedulerOwner)
		if err != nil {
			return err
		}
		defer rows.Close()

		tasklist = nil // reset in case transaction retry
		for rows.Next() {
			var serialized string
			if err = rows.Scan(&serialized); err != nil {
				return err
			}
			tp := tasks.Type.Task.NewBuilder()
			if err = dagjson.Decoder(tp, bytes.NewBufferString(serialized)); err != nil {
				return err
			}
			task := tp.Build().(tasks.Task)
			tasklist = append(tasklist, task)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	var recovered int
	for i := range tasklist {
		err = s.scheduleTask(tasklist[i])
		if err != nil {
			log.Errorw("cannot reschedule task", "taskID", tasklist[i].UUID.String(), "err", err)
		} else {
			recovered++
		}
	}

	return recovered, nil
}

func (s *stateDB) logNextRunTime(taskID string, jobID cron.EntryID) {
	ent := s.cronSched.Entry(jobID)
	if ent.Next.IsZero() {
		log.Errorw("task is no longer scheduled", "taskID", taskID)
	}
	log.Infow("scheduled task", "taskID", taskID, "next_run", ent.Next)
}
