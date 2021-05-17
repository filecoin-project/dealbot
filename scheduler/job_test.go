package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestRunEndJob(t *testing.T) {
	taskCtx, taskCancel := context.WithCancel(context.Background())
	defer taskCancel()

	runChan := make(chan *Job)

	job := &Job{
		ctx:     context.Background(),
		entryID: 42,
		runtime: time.Minute,
		runChan: runChan,
		taskCtx: taskCtx,
	}

	go job.Run()
	var j *Job

	select {
	case j = <-runChan:
	case <-time.After(time.Second):
		t.Fatal("job should have started")
	}

	select {
	case <-j.RunContext().Done():
		t.Fatal("job should not have ended")
	case <-time.After(time.Millisecond):
	}

	j.End()

	select {
	case <-j.RunContext().Done():
	case <-time.After(time.Millisecond):
		t.Fatal("timed out waiting for job to end")
	}
}
