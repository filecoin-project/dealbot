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

func TestRunJob(t *testing.T) {
	runChan := make(chan Job)

	jobInternal := &job{
		entryID: 42,
		runTime: time.Minute,
		runChan: runChan,
	}

	running := make(chan struct{})
	go func() {
		jobInternal.run(false)
		close(running)
	}()

	var j Job

	time.Sleep(time.Second)
	select {
	case <-running:
		t.Fatal("should not be running until run chan is read")
	default:
	}

	select {
	case j = <-runChan:
	case <-time.After(time.Second):
		t.Fatal("job should have started")
	}

	select {
	case <-running:
	case <-time.After(time.Second):
		t.Fatal("job should be running")
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

func TestRunJobWait(t *testing.T) {
	runChan := make(chan Job)
	shutdown := make(chan struct{})

	jobInternal := &job{
		entryID:  42,
		runTime:  time.Minute,
		runChan:  runChan,
		shutdown: shutdown,
	}

	ran := make(chan struct{})
	go func() {
		jobInternal.run(true)
		close(ran)
	}()

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

	select {
	case <-ran:
		t.Fatal("should not have returned from run")
	case <-time.After(time.Second):
	}

	j.End()

	select {
	case <-j.Context.Done():
	default:
		t.Fatal("job should have ended")
	}

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("run should have returned")
	}

	ran = make(chan struct{})
	go func() {
		jobInternal.run(true)
		close(ran)
	}()

	close(shutdown)

	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("run should have returned")
	}

	select {
	case <-j.Context.Done():
	default:
		t.Fatal("job should have ended")
	}
}
