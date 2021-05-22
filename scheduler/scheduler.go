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

var noEnt cron.EntryID

// Scheduler signals when items start and stop
type Scheduler struct {
	cronSched *cron.Cron
	runChan   chan Job

	// Signals used for shutdown
	cancelJobs chan struct{}
	shutdown   chan struct{}
}

// New returns a new Scheduler instance
func New() *Scheduler {
	return newScheduler(cron.New(cron.WithChain(cron.DelayIfStillRunning(cron.DefaultLogger))))
}

// Create a new scheduler that uses a non-standard cron schedule spec that
// contains seconds.
func NewWithSeconds() *Scheduler {
	return newScheduler(cron.New(cron.WithSeconds(), cron.WithChain(cron.DelayIfStillRunning(cron.DefaultLogger))))
}

func newScheduler(cronSched *cron.Cron) *Scheduler {
	cronSched.Start()
	return &Scheduler{
		cronSched:  cronSched,
		runChan:    make(chan Job),
		cancelJobs: make(chan struct{}),
		shutdown:   make(chan struct{}),
	}
}

// RunChan returns a channel that sends jobs to be run
func (s *Scheduler) RunChan() <-chan Job {
	return s.runChan
}

// Add adds a Task to be scheduled.
func (s *Scheduler) Add(cronExp string, task tasks.Task, maxRunTime, scheduleLimit time.Duration, lastRunCount int) (cron.EntryID, error) {
	j := &job{
		runChan:  s.runChan,
		runCount: lastRunCount,
		runTime:  maxRunTime,
		task:     task,

		cancelJobs: s.cancelJobs,
		shutdown:   s.shutdown,
	}
	if cronExp == RunNow {
		j.Queue(func() {})
		return noEnt, nil
	}
	if scheduleLimit != 0 {
		j.expireAt = time.Now().Add(scheduleLimit)
		j.cronSched = s.cronSched
	}
	entID, err := s.cronSched.AddJob(cronExp, j)
	if err != nil {
		return entID, err
	}
	j.entryID = entID
	return entID, nil
}

// Remove removes a Task from the scheduler and stops it if it is running.
func (s *Scheduler) Remove(jobID cron.EntryID) {
	s.cronSched.Remove(jobID)
}

// CLose stops the scheduler, and all its running jobs.  The context passed to
// Stop determines how long the scheduler will wait for it currently running
// tasks to complete before it cancels them.  A nil context means do not wait.
func (s *Scheduler) Close(ctx context.Context) {
	close(s.shutdown)
	stopCtx := s.cronSched.Stop()
	if ctx == nil {
		close(s.cancelJobs)
		<-stopCtx.Done()
	} else {
		select {
		case <-stopCtx.Done():
		case <-ctx.Done():
			close(s.cancelJobs)
			<-stopCtx.Done()
		}
	}
	close(s.runChan)
}
