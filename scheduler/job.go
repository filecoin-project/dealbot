package scheduler

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/robfig/cron/v3"
)

// Job is a set of parameters for running a job
type Job struct {
	ctx       context.Context
	entryID   cron.EntryID
	expireAt  time.Time
	runtime   time.Duration
	run       int32
	runChan   chan<- *Job
	runCtx    context.Context
	runCancel context.CancelFunc
	sched     *cron.Cron
	task      tasks.Task
	taskCtx   context.Context
}

// Send job to worker, wait for worker to signal it is finished.
func (j *Job) Run() {
	// If the schedule has expired remove job from scheduler.
	if !j.expireAt.IsZero() && time.Now().After(j.expireAt) {
		j.sched.Remove(j.entryID)
		return
	}

	atomic.AddInt32(&j.run, 1)
	// Create a context to manage the running time of the current task
	j.runCtx, j.runCancel = context.WithTimeout(j.taskCtx, j.runtime)
	defer j.runCancel()

	// Wait for worker to run this job, or context to cancel
	select {
	case j.runChan <- j:
	case <-j.ctx.Done():
		return
	}
	// Wait for worker to signal that job is completed
	<-j.runCtx.Done()
}

func (j *Job) ID() cron.EntryID {
	return j.entryID
}

func (j *Job) RunContext() context.Context {
	return j.runCtx
}

// Tells the scheduler the job is done
func (j *Job) End() {
	j.runCancel()
}

func (j *Job) RunCount() int {
	return int(atomic.LoadInt32(&j.run))
}

func (j *Job) Task() tasks.Task {
	return j.task
}
