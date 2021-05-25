package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jsonTestDeals = "../../devnet/sample_tasks.json"

func TestLoadTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := ioutil.TempDir("", "testdealbot")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), nil, nil)
	require.NoError(t, err)
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	count, err := state.countTasks(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	rt := tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, "")
	tsk := tasks.Type.Task.New(rt, nil)
	err = state.saveTask(ctx, tsk, "")
	require.NoError(t, err)

	oldCount := count
	count, err = state.countTasks(ctx)
	require.NoError(t, err)
	require.Equal(t, oldCount+1, count)
}

func TestAssignTask(t *testing.T) {
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
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	require.NoError(t, err)

	rt := tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, "")
	task := tasks.Type.Task.New(rt, nil)
	err = state.saveTask(ctx, task, "")
	require.NoError(t, err)

	taskCount, err := state.countTasks(ctx)
	require.NoError(t, err)

	seen := make(map[string]struct{}, taskCount)
	for i := 0; i < taskCount; i++ {
		worker := fmt.Sprintf("tester-%d", i)
		req := tasks.Type.PopTask.Of(worker, tasks.InProgress)
		task, err = state.AssignTask(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, task, "Did not find task to assign")
		require.Equal(t, worker, task.WorkedBy.Must().String(), "should be assigned to correct worker")
		uuid := task.UUID.String()
		_, found := seen[uuid]
		require.False(t, found, "Assigned task that was previously assigned")

		history, err := state.TaskHistory(ctx, uuid)
		require.NoError(t, err)

		assert.Len(t, history, 2)
		assert.Equal(t, tasks.Available, history[0].Status, "wrong status for 1st history")
		assert.Equal(t, tasks.InProgress, history[1].Status, "wrong status for 2nd history")

		seen[uuid] = struct{}{}
	}

	task, err = state.AssignTask(ctx, tasks.Type.PopTask.Of("it's me", tasks.InProgress))
	require.NoError(t, err)
	require.Nil(t, task, "Shoud not be able to assign more tasks")
}

func TestAssignConcurrentTask(t *testing.T) {
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
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	require.NoError(t, err)

	taskCount, err := state.countTasks(ctx)
	require.NoError(t, err)

	release := make(chan struct{})
	assigned := make([]tasks.Task, taskCount)
	errChan := make(chan error)
	t.Log("concurrently assigning", taskCount, "tasks")
	for i := 0; i < taskCount; i++ {
		go func(n int) {
			worker := fmt.Sprintf("worker-%d", n)
			<-release
			req := tasks.Type.PopTask.Of(worker, tasks.InProgress)

			task, err := state.AssignTask(ctx, req)
			if err != nil {
				errChan <- err
				return
			}
			assigned[n] = task
			errChan <- nil
		}(i)
	}

	close(release)
	for i := 0; i < taskCount; i++ {
		err = <-errChan
		require.NoError(t, err)
	}

	for i := 0; i < taskCount; i++ {
		task := assigned[i]
		if task == nil {
			t.Log("did not find task to assign")
			continue
		}
		history, err := state.TaskHistory(ctx, task.UUID.String())
		require.NoError(t, err)

		assert.Len(t, history, 2)
		assert.Equal(t, tasks.Available, history[0].Status, "wrong status for 1st history")
		assert.Equal(t, tasks.InProgress, history[1].Status, "wrong status for 2nd history")
	}
}

func TestAssignTaskWithTag(t *testing.T) {
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
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	//err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	//require.NoError(t, err)

	tasktag := "testtag"
	rt := tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, tasktag)
	task := tasks.Type.Task.New(rt, nil)
	err = state.saveTask(ctx, task, tasktag)
	require.NoError(t, err)

	tasktag = "sometag"
	rt = tasks.Type.RetrievalTask.Of("f0127896", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, tasktag)
	task = tasks.Type.Task.New(rt, nil)
	err = state.saveTask(ctx, task, tasktag)
	require.NoError(t, err)

	// Should not get tagged task with unmatching tags
	worker := "tester"
	req := tasks.Type.PopTask.Of(worker, tasks.InProgress, "foo", "bar")
	require.True(t, req.Tags.Exists(), "Tags does not exist in request")
	task, err = state.AssignTask(ctx, req)
	require.NoError(t, err)
	require.Nil(t, task, "Shoud not get task with tags that do not match search")

	// Should get tagged task with matching tags
	req = tasks.Type.PopTask.Of(worker, tasks.InProgress, "foo", "bar", "testtag")
	task, err = state.AssignTask(ctx, req)
	require.NotNil(t, task, "Did not find tagged task using matching tags")

	// Should get tagged task matching empty tags
	req = tasks.Type.PopTask.Of(worker, tasks.InProgress)
	task, err = state.AssignTask(ctx, req)
	require.NotNil(t, task, "Did not find tagged task using empty tags")

	rt = tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, "")
	task = tasks.Type.Task.New(rt, nil)
	err = state.saveTask(ctx, task, "")
	require.NoError(t, err)

	// Should get untagged task
	req = tasks.Type.PopTask.Of(worker, tasks.InProgress, "foo", "bar")
	task, err = state.AssignTask(ctx, req)
	require.NotNil(t, task, "Did not get untagged task")
}

func TestUpdateTasks(t *testing.T) {
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
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	require.NoError(t, err)

	taskCount, err := state.countTasks(ctx)
	require.NoError(t, err)

	// assign all but one tasks
	var inProgressTasks []tasks.Task
	for i := 0; i < taskCount-1; i++ {
		worker := fmt.Sprintf("tester")
		req := tasks.Type.PopTask.Of(worker, tasks.InProgress)
		task, err := state.AssignTask(ctx, req)
		require.NoError(t, err)
		inProgressTasks = append(inProgressTasks, task)
	}

	allTasks, err := state.GetAll(ctx)
	require.NoError(t, err)

	// find the remaining unassigned task
	var unassignedTask tasks.Task
	for _, task := range allTasks {
		if task.Status == *tasks.Available {
			unassignedTask = task
			break
		}
	}
	require.NotNil(t, unassignedTask)

	exStageDetail := tasks.Type.StageDetails.Of("Doing Stuff", "A good long while")
	workedStageDetail := exStageDetail.WithLog("stuff happened")

	// add a stage name to the last in progress task
	_, err = state.Update(ctx, inProgressTasks[2].GetUUID(),
		tasks.Type.UpdateTask.OfStage(inProgressTasks[2].WorkedBy.Must().String(), tasks.InProgress, "", "Stuff", exStageDetail, 1))
	require.NoError(t, err)

	type statusHistory struct {
		status tasks.Status
		stage  string
		run    int
	}

	testCases := map[string]struct {
		uuid                 string
		updateTaskRequest    tasks.UpdateTask
		expectedStatus       tasks.Status
		expectedErrorMessage string
		expectedStage        string
		expectedStageDetails tasks.StageDetails
		expectedTaskHistory  []statusHistory
		expectedError        error
		expectedRun          int
	}{
		"attempting to work on unassigned task": {
			uuid:              unassignedTask.GetUUID(),
			updateTaskRequest: tasks.Type.UpdateTask.Of("tester", tasks.InProgress, 1),
			expectedError:     ErrNotAssigned,
		},
		"attempting to work on task with another worker": {
			uuid:              inProgressTasks[0].GetUUID(),
			updateTaskRequest: tasks.Type.UpdateTask.Of("tester 2", tasks.Successful, 1),
			expectedError:     ErrWrongWorker,
		},
		"update task status": {
			uuid:              inProgressTasks[0].GetUUID(),
			updateTaskRequest: tasks.Type.UpdateTask.Of(inProgressTasks[0].WorkedBy.Must().String(), tasks.Successful, 1),
			expectedStatus:    tasks.Successful,
			expectedTaskHistory: []statusHistory{
				{tasks.Available, "", 0},
				{tasks.InProgress, "", 0},
				{tasks.Successful, "", 1},
			},
			expectedRun: 1,
		},
		"update stage": {
			uuid:                 inProgressTasks[1].GetUUID(),
			updateTaskRequest:    tasks.Type.UpdateTask.OfStage(inProgressTasks[1].WorkedBy.Must().String(), tasks.InProgress, "", "Stuff", exStageDetail, 1),
			expectedStage:        "Stuff",
			expectedStageDetails: exStageDetail,
			expectedStatus:       tasks.InProgress,
			expectedTaskHistory: []statusHistory{
				{tasks.Available, "", 0},
				{tasks.InProgress, "", 0},
				{tasks.InProgress, "Stuff", 1},
			},
			expectedRun: 1,
		},
		"update stage data within stage": {
			uuid:                 inProgressTasks[2].GetUUID(),
			updateTaskRequest:    tasks.Type.UpdateTask.OfStage(inProgressTasks[2].WorkedBy.Must().String(), tasks.InProgress, "", "Stuff", workedStageDetail, 1),
			expectedStage:        "Stuff",
			expectedStageDetails: workedStageDetail,
			expectedStatus:       tasks.InProgress,
			expectedTaskHistory: []statusHistory{
				{tasks.Available, "", 0},
				{tasks.InProgress, "", 0},
				{tasks.InProgress, "Stuff", 1},
			},
			expectedRun: 1,
		},
		"update error message": {
			uuid:                 inProgressTasks[2].GetUUID(),
			updateTaskRequest:    tasks.Type.UpdateTask.OfStage(inProgressTasks[2].WorkedBy.Must().String(), tasks.Failed, "Something went wrong", "Stuff", workedStageDetail, 1),
			expectedErrorMessage: "Something went wrong",
			expectedStage:        "Stuff",
			expectedStageDetails: workedStageDetail,
			expectedStatus:       tasks.Failed,
			expectedTaskHistory: []statusHistory{
				{tasks.Available, "", 0},
				{tasks.InProgress, "", 0},
				{tasks.InProgress, "Stuff", 1},
				{tasks.Failed, "Stuff", 1},
			},
			expectedRun: 1,
		},
	}

	for testCase, data := range testCases {
		t.Run(testCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			task, err := state.Update(ctx, data.uuid, data.updateTaskRequest)
			if data.expectedError != nil {
				require.Nil(t, task)
				require.EqualError(t, err, data.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, *data.expectedStatus, task.Status)
				if data.expectedErrorMessage == "" {
					require.False(t, task.ErrorMessage.Exists())
				} else {
					require.Equal(t, data.expectedErrorMessage, task.ErrorMessage.Must().String())
				}
				require.Equal(t, data.expectedStage, task.Stage.String())
				if data.expectedStageDetails == nil {
					require.Equal(t, task.CurrentStageDetails.Exists(), false)
				} else {
					require.Equal(t, data.expectedStageDetails, task.CurrentStageDetails.Must())
				}
				taskEvents, err := state.TaskHistory(ctx, data.uuid)
				require.NoError(t, err)
				history := make([]statusHistory, 0, len(taskEvents))
				for _, te := range taskEvents {
					history = append(history, statusHistory{te.Status, te.Stage, te.Run})
				}
				require.Equal(t, data.expectedTaskHistory, history)
				require.Equal(t, data.expectedStage, taskEvents[len(taskEvents)-1].Stage)
				require.Equal(t, data.expectedRun, taskEvents[len(taskEvents)-1].Run)
			}
		})
	}
}

func populateTestTasks(ctx context.Context, jsonTests string, state State) error {
	sampleTaskFile, err := os.Open(jsonTestDeals)
	if err != nil {
		return err
	}
	defer sampleTaskFile.Close()
	sampleTasks, err := ioutil.ReadAll(sampleTaskFile)
	if err != nil {
		return err
	}
	byTask := make([]json.RawMessage, 0)
	if err = json.Unmarshal(sampleTasks, &byTask); err != nil {
		return err
	}
	for _, task := range byTask {
		rtp := tasks.Type.RetrievalTask.NewBuilder()
		if err = dagjson.Decoder(rtp, bytes.NewBuffer(task)); err != nil {
			stp := tasks.Type.StorageTask.NewBuilder()
			if err = dagjson.Decoder(stp, bytes.NewBuffer(task)); err != nil {
				return fmt.Errorf("could not decode sample task as either storage or retrieval %s: %w", task, err)
			}
			if _, err = state.NewStorageTask(ctx, stp.Build().(tasks.StorageTask)); err != nil {
				return err
			}
		} else {
			if _, err = state.NewRetrievalTask(ctx, rtp.Build().(tasks.RetrievalTask)); err != nil {
				return err
			}
		}
	}
	return nil
}

func makeKey() (crypto.PrivKey, error) {
	const identity = ".dealbot.test.key"

	// make a new identity
	pr, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		return nil, err
	}
	return pr, nil
}
