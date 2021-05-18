package scheduler

import (
	"context"
	"time"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/robfig/cron/v3"
)

// job is the scheduler's internal record of a scheduled job
type job struct {
	cronSched *cron.Cron
	entryID   cron.EntryID
	expireAt  time.Time
	runTime   time.Duration
	runCount  int
	runChan   chan<- Job
	task      tasks.Task

	// Signals used for shutdown
	cancelJobs chan struct{}
	shutdown   <-chan struct{}
}

// Job is information that is output when a job runs
type Job struct {
	Context  context.Context
	End      context.CancelFunc
	ID       cron.EntryID
	RunCount int
	Task     tasks.Task
}

// Send job to worker, wait for worker to signal it is finished.
func (j *job) Run() {
	// If the schedule has expired remove job from scheduler.
	if !j.expireAt.IsZero() && time.Now().After(j.expireAt) {
		j.cronSched.Remove(j.entryID)
		return
	}

	j.runCount++

	// Create a context to manage the running time of the current task
	runCtx, runCancel := context.WithTimeout(context.Background(), j.runTime)
	defer runCancel()

	jobInfo := Job{
		Context:  runCtx,
		End:      runCancel,
		ID:       j.entryID,
		RunCount: j.runCount,
		Task:     j.task,
	}

	// Wait for worker to run this job, or shutdown signal
	select {
	case j.runChan <- jobInfo:
	case <-j.shutdown:
		return
	}
	// Wait for worker to signal that job is completed or jobs to be
	// canceled. Do not wait for shutdown here so that running jobs can finish.
	select {
	case <-runCtx.Done():
	case <-j.cancelJobs:
	}
}
