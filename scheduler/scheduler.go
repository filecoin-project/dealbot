/*
Package scheduler sends a job on a channel when it is time to run it.

The scheduler takes a Task and cron expression.  At the tasks's start time, the
task is sent to the run channel. This allows the control of task executon
outside of the scheduler.  When the task is finished, the executor calls
Task.End() to tell the scheduler the task is finished.
*/
package scheduler

import (
	"context"
	"time"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/robfig/cron/v3"
)

const (
	RunNow = "NOW"
)

// Scheduler signals when items start and stop
type Scheduler struct {
	ctx     context.Context
	runChan chan *Job
	sched   *cron.Cron

	taskCtx     context.Context
	cancelTasks context.CancelFunc
}

// New returns a new Scheduler instance
func New(ctx context.Context) *Scheduler {
	return newScheduler(ctx, cron.New(cron.WithChain(cron.DelayIfStillRunning(cron.DefaultLogger))))
}

func NewWithSeconds(ctx context.Context) *Scheduler {
	return newScheduler(ctx, cron.New(cron.WithSeconds(), cron.WithChain(cron.DelayIfStillRunning(cron.DefaultLogger))))
}

func newScheduler(ctx context.Context, sched *cron.Cron) *Scheduler {
	sched.Start()
	s := &Scheduler{
		ctx:     ctx,
		runChan: make(chan *Job),
		sched:   sched,
	}
	s.taskCtx, s.cancelTasks = context.WithCancel(context.Background())
	return s
}

// RunChan returns a channel that sends jobs to be run
func (s *Scheduler) RunChan() <-chan *Job {
	return s.runChan
}

// Add adds a Task to be scheduled.
func (s *Scheduler) Add(cronExp string, task tasks.Task, maxRunTime, scheduleLife time.Duration) (cron.EntryID, error) {
	job := &Job{
		ctx:     s.ctx,
		runtime: maxRunTime,
		runChan: s.runChan,
		task:    task,
		taskCtx: s.taskCtx,
	}
	if cronExp == RunNow {
		job.Run()
		var noEnt cron.EntryID
		return noEnt, nil
	}
	if scheduleLife != 0 {
		job.sched = s.sched
		job.expireAt = time.Now().Add(scheduleLife)
	}
	entID, err := s.sched.AddJob(cronExp, job)
	if err != nil {
		return entID, err
	}
	job.entryID = entID
	return entID, nil
}

// Remove removes a Task from the scheduler and stops it if it is running.
func (s *Scheduler) Remove(jobID cron.EntryID) {
	s.sched.Remove(jobID)
}

// Stop stops the scheduler, and all its running jobs.  The context passed to
// Stop determines how long the scheduler will wait for it currently running
// tasks to complete before it cancels them.  A nil context means no wait.
func (s *Scheduler) Stop(ctx context.Context) {
	stopCtx := s.sched.Stop()
	if ctx == nil {
		s.cancelTasks()
		<-stopCtx.Done()
	} else {
		select {
		case <-stopCtx.Done():
		case <-ctx.Done():
			s.cancelTasks()
			<-stopCtx.Done()
		}
	}
	close(s.runChan)
}
