package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	everySecond   = "* * * * * *"
	maxRunTime    = 500 * time.Millisecond
	scheduleLimit = 5 * time.Second
)

func TestScheduleTask(t *testing.T) {
	s := NewWithSeconds()

	// Add task to scheduler
	jid, err := s.Add(everySecond, nil, maxRunTime, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	var noEnt cron.Entry
	ent := s.cronSched.Entry(jid)
	if ent == noEnt {
		t.Fatal("job was not scheduled")
	}

	// Test that job starts
	var job Job
	select {
	case job = <-s.RunChan():
		if job.ID != ent.ID {
			t.Fatal("wrong ID, got", job.ID, "expected", jid)
		}
		t.Log("started", job.ID, "run", job.RunCount)
	case <-time.After(time.Second + maxRunTime):
		t.Fatal("timed out waiting for job to start")
	}

	if job.RunCount != 1 {
		t.Errorf("job has wrong run count, expected 1, got %d", job.RunCount)
	}

	// Test that job stops
	select {
	case <-job.Context.Done():
		t.Log("stopped", job.ID, "run", job.RunCount)
	case <-time.After(time.Second + maxRunTime):
		t.Fatal("timed out waiting for job to stop")
	}

	// Test that job starts again
	select {
	case job = <-s.RunChan():
		if job.ID != ent.ID {
			t.Fatal("wrong ID, got", job.ID, "expected", jid)
		}
		t.Log("started", job.ID, "run", job.RunCount)
	case <-time.After(time.Second + maxRunTime):
		t.Fatal("timed out waiting for job to start")
	}

	if job.RunCount != 2 {
		t.Errorf("job has wrong run count, expected 2, got %d", job.RunCount)
	}

	job.End()

	// Test that job stops immediately
	select {
	case <-job.Context.Done():
		t.Log("stopped", job.ID, "run", job.RunCount)
	default:
		t.Fatal("job should have stopped")
	}

	// Test removing a job
	s.Remove(jid)
	ent = s.cronSched.Entry(jid)
	if ent != noEnt {
		t.Fatal("job was not removed")
	}
	t.Log("removed job", jid)

	// Add job to scheduler
	jid, err = s.Add(everySecond, nil, maxRunTime, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("added job", jid)

	// Wait for job to start
	select {
	case job = <-s.RunChan():
		if job.ID != jid {
			t.Fatal("started unexpected job")
		}
		t.Log("started", job.ID, "run", job.RunCount)
	case <-time.After(time.Second + maxRunTime):
		t.Fatal("timed out waiting for job to start")
	}

	// Shutdown with task cancellation
	t.Log("shutting down scheduler")
	s.Close(nil)
	t.Log("scheduler is shutdown")

	select {
	case <-job.Context.Done():
		t.Log("stopped", job.ID, "run", job.RunCount)
	case <-time.After(time.Second):
		t.Error("did not cancel task immediately")
	}
}

func TestScheduleLimit(t *testing.T) {
	s := NewWithSeconds()

	// Add job to scheduler
	jid, err := s.Add(everySecond, nil, maxRunTime, scheduleLimit, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("added job", jid, "scheduling should end in", scheduleLimit)

	// Continue running until deadline.
	deadline := time.After(scheduleLimit + time.Second)
	var hitDeadline bool
	var job Job
	var lastRun int
	for !hitDeadline {
		select {
		case job = <-s.RunChan():
			if job.RunCount != lastRun+1 {
				t.Error("expected run = last + 1")
			}
			lastRun++
			t.Log("ran", job.ID, "run", job.RunCount)
			job.End()
		case <-deadline:
			hitDeadline = true
		}
	}
	// Check for any more runs after deadline.
	select {
	case job = <-s.RunChan():
		t.Error("job was scheduled after deadline")
		job.End()
		s.Remove(job.ID)
	case <-time.After(2 * time.Second):
	}

	// Shutdown with task cancellation
	t.Log("shutting down scheduler")
	s.Close(nil)
	t.Log("scheduler is shutdown")

	select {
	case <-job.Context.Done():
		t.Log("stopped", job.ID, "run", job.RunCount)
	case <-time.After(time.Second):
		t.Error("did not cancel task immediately")
	}
}

func TestScheduleRunNow(t *testing.T) {
	s := NewWithSeconds()

	runChan := s.RunChan()

	wg := new(sync.WaitGroup)
	worker := func() {
		defer wg.Done()
		for job := range runChan {
			t.Log("running job")
			select {
			case <-time.After(5 * time.Second):
				t.Log("finished job")
			case <-job.Context.Done():
				t.Log("canceled job")
			}
			job.End()
		}
	}

	wg.Add(1)
	go worker()

	for i := 0; i < 4; i++ {
		t.Log("Adding job", i)
		_, err := s.Add(RunNow, nil, 30*time.Second, 0, 0)
		if err != nil {
			t.Error(err)
		} else {
			t.Log("Added job", i)
		}
		t.Log("")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Log("closing scheduler")
	s.Close(ctx)
	wg.Wait()
}
