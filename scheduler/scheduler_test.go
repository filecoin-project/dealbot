package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	everySecond   = "* * * * * *"
	maxRunTime    = 200 * time.Millisecond
	scheduleLimit = 2 * time.Second
)

func TestScheduleTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewWithSeconds(ctx)

	// Add task to scheduler
	jid, err := s.Add(everySecond, nil, maxRunTime, 0)
	if err != nil {
		t.Fatal(err)
	}

	var noEnt cron.Entry
	ent := s.sched.Entry(jid)
	if ent == noEnt {
		t.Fatal("job was not scheduled")
	}

	// Test that job starts
	var job *Job
	select {
	case job = <-s.RunChan():
		if job.ID() != ent.ID {
			t.Fatal("wrong ID, got", job.ID(), "expected", jid)
		}
		t.Log("started", job.ID(), "run", job.RunCount())
	case <-time.After(time.Second + maxRunTime):
		t.Fatal("timed out waiting for job to start")
	}

	if job.RunCount() != 1 {
		t.Errorf("job has wrong run count, expected 1, got %d", job.RunCount())
	}

	// Test that job stops
	select {
	case <-job.RunContext().Done():
		t.Log("stopped", job.ID(), "run", job.RunCount())
	case <-time.After(time.Second + maxRunTime):
		t.Fatal("timed out waiting for job to stop")
	}

	// Test that job starts again
	select {
	case job = <-s.RunChan():
		if job.ID() != ent.ID {
			t.Fatal("wrong ID, got", job.ID(), "expected", jid)
		}
		t.Log("started", job.ID(), "run", job.RunCount())
	case <-time.After(time.Second + maxRunTime):
		t.Fatal("timed out waiting for job to start")
	}

	if job.RunCount() != 2 {
		t.Errorf("job has wrong run count, expected 2, got %d", job.RunCount())
	}

	job.End()

	// Test that job stops immediately
	select {
	case <-job.RunContext().Done():
		t.Log("stopped", job.ID(), "run", job.RunCount())
	case <-time.After(time.Millisecond):
		t.Fatal("timed out waiting for job to stop")
	}

	// Test removing a job
	s.Remove(jid)
	ent = s.sched.Entry(jid)
	if ent != noEnt {
		t.Fatal("job was not removed")
	}
	t.Log("removed job", jid)

	// Add job to scheduler
	jid, err = s.Add(everySecond, nil, maxRunTime, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("added job", jid)

	// Wait for job to start
	select {
	case job = <-s.RunChan():
		if job.ID() != jid {
			t.Fatal("started unexpected job")
		}
		t.Log("started", job.ID(), "run", job.RunCount())
	case <-time.After(time.Second + maxRunTime):
		t.Fatal("timed out waiting for job to start")
	}

	// Shutdown with task cancellation
	t.Log("shutting down scheduler")
	s.Stop(nil)
	t.Log("scheduler is shutdown")

	select {
	case <-job.RunContext().Done():
		t.Log("stopped", job.ID(), "run", job.RunCount())
	case <-time.After(time.Second):
		t.Error("did not cancel task immediately")
	}
}

func TestScheduleLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewWithSeconds(ctx)

	// Add job to scheduler
	jid, err := s.Add(everySecond, nil, maxRunTime, scheduleLimit)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("added job", jid, "scheduling should end in", scheduleLimit)

	// Continue running until deadline.
	deadline := time.After(scheduleLimit + time.Second)
	var hitDeadline bool
	var job *Job
	for !hitDeadline {
		select {
		case job = <-s.RunChan():
			t.Log("ran", job.ID(), "run", job.RunCount())
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
		s.Remove(job.ID())
	case <-time.After(2 * time.Second):
	}

	// Shutdown with task cancellation
	t.Log("shutting down scheduler")
	s.Stop(nil)
	t.Log("scheduler is shutdown")

	select {
	case <-job.RunContext().Done():
		t.Log("stopped", job.ID(), "run", job.RunCount())
	case <-time.After(time.Second):
		t.Error("did not cancel task immediately")
	}
}