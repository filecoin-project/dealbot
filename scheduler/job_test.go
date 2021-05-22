package scheduler

import (
	"testing"
	"time"
)

func TestRunEndJob(t *testing.T) {
	runChan := make(chan Job)

	jobInternal := &job{
		entryID: 42,
		runTime: time.Minute,
		runChan: runChan,
	}

	go jobInternal.Run()
	var j Job

	select {
	case j = <-runChan:
	case <-time.After(time.Second):
		t.Fatal("job should have started")
	}

	select {
	case <-j.Context.Done():
		t.Fatal("job should not have ended")
	default:
	}

	j.End()

	select {
	case <-j.Context.Done():
	default:
		t.Fatal("job should have ended")
	}
}

func TestQueue(t *testing.T) {
	runChan := make(chan Job)

	jobInternal := &job{
		entryID: 42,
		runTime: time.Minute,
		runChan: runChan,
	}

	queueDone := make(chan struct{})
	jobDone := make(chan struct{})
	go func() {
		jobInternal.Queue(func() {
			close(jobDone)
		})
		close(queueDone)
	}()

	var j Job

	time.Sleep(time.Second)
	select {
	case <-queueDone:
		t.Fatal("should not finish queue until run chan is read")
	default:
	}
	select {
	case <-jobDone:
		t.Fatal("done callback not called till job is finished")
	default:
	}

	select {
	case j = <-runChan:
	case <-time.After(time.Second):
		t.Fatal("job should have started")
	}

	select {
	case <-queueDone:
	case <-time.After(time.Second):
		t.Fatal("job should have finished queueing")
	}
	select {
	case <-jobDone:
		t.Fatal("done callback not called till job is finished")
	default:
	}
	select {
	case <-j.Context.Done():
		t.Fatal("job should not have ended")
	default:
	}

	j.End()

	select {
	case <-j.Context.Done():
	default:
		t.Fatal("job should have ended")
	}

	select {
	case <-jobDone:
	case <-time.After(time.Second):
		t.Fatal("job should have ended")
	}
}
