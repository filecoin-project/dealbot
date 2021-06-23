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

const schedulerOwner = "dealbot_scheduler"

var noEnt cron.EntryID

// job is the scheduler's internal record of a scheduled job
type job struct {
	entryID   cron.EntryID
	expireAt  time.Time
	idChan    chan cron.EntryID
	limit     time.Duration
	schedule  string
	sdb       *stateDB
	runNotice chan string
	taskID    string
}

// Run creates a copy of the task without the schedule for PopTask
func (j *job) Run() {
	if j.entryID == noEnt {
		j.entryID = <-j.idChan
	}
	newTaskID := j.sdb.runTask(j.taskID, j.schedule, j.expireAt, j.limit, j.entryID)
	if j.runNotice != nil && newTaskID != "" {
		select {
		case j.runNotice <- newTaskID:
		default:
		}
	}
}

func (s *stateDB) scheduleTask(task tasks.Task, runNotice chan string) error {
	taskID := task.UUID.String()
	taskSchedule, limit := getTaskSchedule(task)
	if taskSchedule == "" {
		log.Infow("task became unscheduled, resetting task", "taskID", taskID)
		s.unassignScheduledTask(taskID)
		return nil
	}

	log.Infow("scheduling task", "uuid", taskID, "schedule", taskSchedule, "schedule_limit", limit)

	j := &job{
		idChan:    make(chan cron.EntryID, 1),
		limit:     limit,
		schedule:  taskSchedule,
		sdb:       s,
		runNotice: runNotice,
		taskID:    taskID,
	}
	if limit != 0 {
		j.limit = limit
		j.expireAt = time.Now().Add(limit)
	}
	entID, err := s.cronSched.AddJob(taskSchedule, j)
	if err != nil {
		return fmt.Errorf("invalid schedule specification %q: %s", taskSchedule, err)
	}
	j.idChan <- entID

	s.logNextRunTime(taskID, entID)
	return nil
}

func (s *stateDB) runTask(taskID, schedule string, expireAt time.Time, limit time.Duration, jobID cron.EntryID) string {
	task, tag, err := s.getWithTag(context.Background(), taskID)
	if err != nil {
		log.Errorw("cannot load scheduled task from database", "taskID", taskID, "err", err)
		return ""
	}

	// If task removed then stop scheduling it
	if task == nil {
		log.Infow("scheduled task removed", "taskID", taskID)
		s.cronSched.Remove(jobID)
		return ""
	}

	// If schedule changed, unassign and let task be rescheduled
	taskSchedule, schedLimit := getTaskSchedule(task)
	if taskSchedule != schedule || schedLimit != limit {
		log.Infow("task scheduled changed, resetting task", "taskID", taskID)
		s.unassignScheduledTask(taskID)
		s.cronSched.Remove(jobID)
		return ""
	}

	// If the schedule has expired remove job from scheduler
	if !expireAt.IsZero() && time.Now().After(expireAt) {
		log.Infow("scheduling expired for task", "taskID", taskID)
		s.cronSched.Remove(jobID)
		return ""
	}

	newTaskID, err := s.createRunableTask(task, tag)
	if err != nil {
		log.Errorw("cannot create runable task", "scheduledTaskID", taskID, "err", err)
		return ""
	}

	log.Infow("created new runable task from scheduled task", "runableTaskID", newTaskID, "scheduledTaskID", taskID)
	s.logNextRunTime(taskID, jobID)
	return newTaskID
}

func (s *stateDB) createRunableTask(task tasks.Task, tag string) (string, error) {
	newTaskID := uuid.New().String()
	runableTask := task.MakeRunable(newTaskID)

	err := s.saveTask(context.Background(), runableTask, tag)
	if err != nil {
		return "", err
	}
	return newTaskID, nil
}

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

func (s *stateDB) recoverScheduledTasks(ctx context.Context) error {
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
		return err
	}

	for i := range tasklist {
		err = s.scheduleTask(tasklist[i], nil)
		if err != nil {
			log.Errorw("cannot reschedule task", "taskID", tasklist[i].UUID.String(), "err", err)
		}
	}

	log.Infow("recovered scheduled tasks", "task_count", len(tasklist))
	return nil
}

func (s *stateDB) logNextRunTime(taskID string, jobID cron.EntryID) {
	ent := s.cronSched.Entry(jobID)
	if ent.Next.IsZero() {
		log.Errorw("task is no longer scheduled", "taskID", taskID)
	}
	log.Infow("scheduled task", "taskID", taskID, "next_run", ent.Next)
}

func hasSchedule(task tasks.Task) bool {
	if t := task.RetrievalTask; t.Exists() {
		if sch := t.Must().Schedule; sch.Exists() {
			return sch.Must().String() != ""
		}
	}
	return false
}

func getTaskSchedule(task tasks.Task) (string, time.Duration) {
	var schedule, limit string
	var duration time.Duration

	if t := task.RetrievalTask; t.Exists() {
		if sch := t.Must().Schedule; sch.Exists() {
			schedule = sch.Must().String()
			if lim := t.Must().ScheduleLimit; lim.Exists() {
				limit = lim.Must().String()
			}
		}
	}

	if t := task.StorageTask; t.Exists() {
		if sch := t.Must().Schedule; sch.Exists() {
			schedule = sch.Must().String()
			if lim := t.Must().ScheduleLimit; lim.Exists() {
				limit = lim.Must().String()
			}
		}
	}

	if schedule != "" && limit != "" {
		var err error
		duration, err = time.ParseDuration(limit)
		if err != nil {
			log.Errorw("task has invalid value for ScheduleLimit", "uuid", task.UUID.String(), "err", err)
		}
	}

	return schedule, duration
}
