package state

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var scheduledTasks = `[{"Miner":"t01000","PayloadCID":"bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36","CARExport":false,"Schedule":"*/5 * * * * *","ScheduleLimit":"11s"}]`

func TestScheduledTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := ioutil.TempDir("", "testdealbot")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	key, err := makeKey()
	require.NoError(t, err)

	runNotice := make(chan string, 1)

	stateInterface, err := newStateDBWithNotify(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), "", key, nil, runNotice)
	require.NoError(t, err)
	state := stateInterface.(*stateDB)

	// Replace normal scheduler with one that accepts seconds
	state.cronSched.Stop()
	state.cronSched = cron.New(cron.WithSeconds())
	state.cronSched.Start()

	err = populateTestTasks(ctx, bytes.NewReader([]byte(scheduledTasks)), stateInterface)
	require.NoError(t, err)

	worker := "test_worker"
	task, scheduled, err := state.popTask(ctx, worker, tasks.InProgress, nil)
	require.NoError(t, err)
	require.Nil(t, task, "should not find unassigned task")

	t.Log("waiting for scheduled task 1st generation")
	var newTaskID string
	select {
	case <-time.After(10 * time.Second):
		t.Fatal("scheduled task did not run")
	case newTaskID = <-runNotice:
		t.Log("scheduler generated task", newTaskID)
	}

	newTask, err := state.Get(ctx, newTaskID)
	require.NoError(t, err)
	require.NotNil(t, newTask, "Did not find new generated task")
	assert.Equal(t, newTaskID, newTask.UUID.String(), "wrong uuid for new task")

	sch, _ := newTask.Schedule()
	assert.Equal(t, "", sch, "new task should not have schedule")

	t.Log("popping generated task")
	task, scheduled, err = state.popTask(ctx, worker, tasks.InProgress, nil)
	require.NoError(t, err)
	require.NotNil(t, task, "Did not find runable task")
	require.False(t, scheduled, "should not have found scheduled task")
	require.Equal(t, worker, task.WorkedBy.Must().String(), "should be assigned to test_worker")
	runCount, err := task.RunCount.AsInt()
	require.NoError(t, err)
	assert.Equal(t, 0, int(runCount))
	t.Log("popped task assigned to", worker)

	t.Log("waiting for scheduled task 2nd generation")
	select {
	case <-time.After(10 * time.Second):
		t.Fatal("scheduled task did not run again")
	case newTaskID = <-runNotice:
		t.Log("scheduler generated task", newTaskID)
	}
	newTask, err = state.Get(ctx, newTaskID)
	require.NoError(t, err)
	require.NotNil(t, newTask, "Did not find new generated task")
	assert.Equal(t, newTaskID, newTask.UUID.String(), "wrong uuid for new task")

	t.Log("popping next generated task")
	task, scheduled, err = state.popTask(ctx, worker, tasks.InProgress, nil)
	require.NoError(t, err)
	require.NotNil(t, task, "Did not find runable task")
	require.False(t, scheduled, "should not have found scheduled task")
	require.Equal(t, worker, task.WorkedBy.Must().String(), "should be assigned to test_worker")
	runCount, err = task.RunCount.AsInt()
	require.NoError(t, err)
	assert.Equal(t, 1, int(runCount))
	t.Log("popped another task assigned to", worker)

	t.Log("waiting to see that task is not scheduled for 3nd generation")
	// Wait for any additional scheduling
	timeout := time.After(6 * time.Second)
	for waited := false; !waited; {
		select {
		case <-timeout:
			waited = true
		case newTaskID = <-runNotice:
		}
	}
	// Make sure there is no more scheduling
	select {
	case <-time.After(6 * time.Second):
	case newTaskID = <-runNotice:
		t.Error("scheduler should not have generated another task")
	}

}
