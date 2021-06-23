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

var scheduledTasks = `[{"Miner":"t01000","PayloadCID":"bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36","CARExport":false,"Schedule":"*/5 * * * * *"}]`

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

	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), key, nil)
	require.NoError(t, err)
	state := stateInterface.(*stateDB)

	state.cronSched.Stop()
	state.cronSched = cron.New(cron.WithSeconds())
	state.cronSched.Start()

	err = populateTestTasks(ctx, bytes.NewReader([]byte(scheduledTasks)), stateInterface)
	require.NoError(t, err)

	t.Log("popping scheduled task")
	worker := "test_worker"
	task, scheduled, err := state.popTask(ctx, worker, tasks.InProgress, nil)
	require.NoError(t, err)
	require.NotNil(t, task, "Did not find task to schedule")
	require.True(t, scheduled, "Task with schedule was not scheduled")

	taskID := task.UUID.String()
	t.Log("scheduling task", taskID)

	runNotice := make(chan string, 1)
	err = state.scheduleTask(task, runNotice)
	require.NoError(t, err)

	var newTaskID string
	select {
	case <-time.After(10 * time.Second):
		t.Fatal("scheduled task did not run")
	case newTaskID = <-runNotice:
		t.Log("scheduled task", taskID, "generated task", newTaskID)
	}
	newTask, err := state.Get(ctx, newTaskID)
	require.NoError(t, err)
	require.NotNil(t, newTask, "Did not find new generated task")
	assert.Equal(t, newTaskID, newTask.UUID.String(), "wrong uuid for new task")

	sch, _ := getTaskSchedule(newTask)
	assert.Equal(t, "", sch, "new task should not have schedule")

	t.Log("popping generated task")
	task, scheduled, err = state.popTask(ctx, worker, tasks.InProgress, nil)
	require.NoError(t, err)
	require.NotNil(t, task, "Did not find runable task")
	require.False(t, scheduled, "should not have found scheduled task")
	require.Equal(t, worker, task.WorkedBy.Must().String(), "should be assigned to test_worker")
	runCount, err := task.RunCount.AsInt()
	require.NoError(t, err)
	assert.Equal(t, 1, int(runCount))
	t.Log("popped task assigned to", worker)

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("scheduled task did not run again")
	case newTaskID = <-runNotice:
		t.Log("scheduled task", taskID, "generated task", newTaskID)
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
	assert.Equal(t, 2, int(runCount))
	t.Log("popped another task assigned to", worker)
}
