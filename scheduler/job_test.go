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
