package state

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/google/uuid"
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

	MigrationsDir = "migrations"
	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), nil, nil)
	require.NoError(t, err)
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	count, err := state.countTasks(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	err = state.saveTask(ctx, &tasks.Task{
		UUID:   uuid.New().String()[:8],
		Status: tasks.Available,
		RetrievalTask: &tasks.RetrievalTask{
			Miner:      "t01000",
			PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
			CARExport:  false,
		},
	})
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

	MigrationsDir = "migrations"
	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), key, nil)
	require.NoError(t, err)
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	require.NoError(t, err)

	task := &tasks.Task{
		UUID:   uuid.New().String()[:8],
		Status: tasks.Available,
		RetrievalTask: &tasks.RetrievalTask{
			Miner:      "t01000",
			PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
			CARExport:  false,
		},
	}
	err = state.saveTask(ctx, task)
	require.NoError(t, err)

	taskCount, err := state.countTasks(ctx)
	require.NoError(t, err)

	seen := make(map[string]struct{}, taskCount)
	for i := 0; i < taskCount; i++ {
		worker := fmt.Sprintf("tester-%d", i)
		req := client.PopTaskRequest{
			Status:   tasks.InProgress,
			WorkedBy: worker,
		}
		task, err = state.AssignTask(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, task, "Did not find task to assign")
		require.Equal(t, worker, task.WorkedBy, "should be assigned to correct worker")
		_, found := seen[task.UUID]
		require.False(t, found, "Assigned task that was previously assigned")

		history, err := state.TaskHistory(ctx, task.UUID)
		require.NoError(t, err)

		assert.Len(t, history, 2)
		assert.Equal(t, tasks.Available, history[0].Status, "wrong status for 1st history")
		assert.Equal(t, tasks.InProgress, history[1].Status, "wrong status for 2nd history")

		seen[task.UUID] = struct{}{}
	}

	task, err = state.AssignTask(ctx, client.PopTaskRequest{
		Status:   tasks.InProgress,
		WorkedBy: "it's me",
	})
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

	MigrationsDir = "migrations"
	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), key, nil)
	require.NoError(t, err)
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	require.NoError(t, err)

	taskCount, err := state.countTasks(ctx)
	require.NoError(t, err)

	release := make(chan struct{})
	assigned := make([]*tasks.Task, taskCount)
	errChan := make(chan error)
	t.Log("concurrently assigning", taskCount, "tasks")
	for i := 0; i < taskCount; i++ {
		go func(n int) {
			worker := fmt.Sprintf("worker-%d", n)
			<-release
			req := client.PopTaskRequest{
				Status:   tasks.InProgress,
				WorkedBy: worker,
			}

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
		history, err := state.TaskHistory(ctx, task.UUID)
		require.NoError(t, err)

		assert.Len(t, history, 2)
		assert.Equal(t, tasks.Available, history[0].Status, "wrong status for 1st history")
		assert.Equal(t, tasks.InProgress, history[1].Status, "wrong status for 2nd history")
	}
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

	MigrationsDir = "migrations"
	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), key, nil)
	require.NoError(t, err)
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")

	err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	require.NoError(t, err)

	taskCount, err := state.countTasks(ctx)
	require.NoError(t, err)

	// assign all but one tasks
	var inProgressTasks []*tasks.Task
	for i := 0; i < taskCount-1; i++ {
		worker := fmt.Sprintf("tester")
		req := client.PopTaskRequest{
			Status:   tasks.InProgress,
			WorkedBy: worker,
		}
		task, err := state.AssignTask(ctx, req)
		require.NoError(t, err)
		inProgressTasks = append(inProgressTasks, task)
	}

	allTasks, err := state.GetAll(ctx)
	require.NoError(t, err)

	// find the remaining unassigned task
	var unassignedTask *tasks.Task
	for _, task := range allTasks {
		if task.Status == tasks.Available {
			unassignedTask = task
			break
		}
	}
	require.NotNil(t, unassignedTask)

	// add a stage name to the last in progress task
	_, err = state.Update(ctx, inProgressTasks[2].UUID, client.UpdateTaskRequest{
		Status: tasks.InProgress,
		Stage:  "Stuff",
		CurrentStageDetails: &tasks.StageDetails{
			Description:      "Doing Stuff",
			ExpectedDuration: "A good long while",
		},
		WorkedBy: inProgressTasks[2].WorkedBy,
	})
	require.NoError(t, err)

	type statusHistory struct {
		status tasks.Status
		stage  string
	}

	testCases := map[string]struct {
		uuid                 string
		updateTaskRequest    client.UpdateTaskRequest
		expectedStatus       tasks.Status
		expectedStage        string
		expectedStageDetails *tasks.StageDetails
		expectedTaskHistory  []statusHistory
		expectedError        error
	}{
		"attempting to work on unassigned task": {
			uuid: unassignedTask.UUID,
			updateTaskRequest: client.UpdateTaskRequest{
				Status:   tasks.InProgress,
				WorkedBy: "tester",
			},
			expectedError: ErrNotAssigned,
		},
		"attempting to work on task with another worker": {
			uuid: inProgressTasks[0].UUID,
			updateTaskRequest: client.UpdateTaskRequest{
				Status:   tasks.Successful,
				WorkedBy: "tester 2",
			},
			expectedError: ErrWrongWorker,
		},
		"update task status": {
			uuid: inProgressTasks[0].UUID,
			updateTaskRequest: client.UpdateTaskRequest{
				Status:   tasks.Successful,
				WorkedBy: inProgressTasks[0].WorkedBy,
			},
			expectedStatus: tasks.Successful,
			expectedTaskHistory: []statusHistory{
				{tasks.Available, ""},
				{tasks.InProgress, ""},
				{tasks.Successful, ""},
			},
		},
		"update stage": {
			uuid: inProgressTasks[1].UUID,
			updateTaskRequest: client.UpdateTaskRequest{
				Status: tasks.InProgress,
				Stage:  "Stuff",
				CurrentStageDetails: &tasks.StageDetails{
					Description:      "Doing Stuff",
					ExpectedDuration: "A good long while",
				},
				WorkedBy: inProgressTasks[1].WorkedBy,
			},
			expectedStage: "Stuff",
			expectedStageDetails: &tasks.StageDetails{
				Description:      "Doing Stuff",
				ExpectedDuration: "A good long while",
			},
			expectedStatus: tasks.InProgress,
			expectedTaskHistory: []statusHistory{
				{tasks.Available, ""},
				{tasks.InProgress, ""},
				{tasks.InProgress, "Stuff"},
			},
		},
		"update stage data within stage": {
			uuid: inProgressTasks[2].UUID,
			updateTaskRequest: client.UpdateTaskRequest{
				Status: tasks.InProgress,
				Stage:  "Stuff",
				CurrentStageDetails: &tasks.StageDetails{
					Description:      "Doing Stuff",
					ExpectedDuration: "A good long while",
					Logs: []*tasks.Log{
						{
							Log: "stuff happened",
						},
					},
				},
				WorkedBy: inProgressTasks[2].WorkedBy,
			},
			expectedStage: "Stuff",
			expectedStageDetails: &tasks.StageDetails{
				Description:      "Doing Stuff",
				ExpectedDuration: "A good long while",
				Logs: []*tasks.Log{
					{
						Log: "stuff happened",
					},
				},
			},
			expectedStatus: tasks.InProgress,
			expectedTaskHistory: []statusHistory{
				{tasks.Available, ""},
				{tasks.InProgress, ""},
				{tasks.InProgress, "Stuff"},
			},
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
				require.Equal(t, data.expectedStatus, task.Status)
				require.Equal(t, data.expectedStage, task.Stage)
				require.Equal(t, data.expectedStageDetails, task.CurrentStageDetails)
				taskEvents, err := state.TaskHistory(ctx, data.uuid)
				require.NoError(t, err)
				history := make([]statusHistory, 0, len(taskEvents))
				for _, te := range taskEvents {
					history = append(history, statusHistory{te.Status, te.Stage})
				}
				require.Equal(t, data.expectedTaskHistory, history)
				require.Equal(t, data.expectedStage, taskEvents[len(taskEvents)-1].Stage)
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
		rt := tasks.RetrievalTask{}
		if err = json.Unmarshal(task, &rt); err != nil {
			st := tasks.StorageTask{}
			if err = json.Unmarshal(task, &st); err != nil {
				return fmt.Errorf("could not decode sample task as either storage or retrieval %s: %w", task, err)
			}
			if _, err = state.NewStorageTask(ctx, &st); err != nil {
				return err
			}
		} else {
			if _, err = state.NewRetrievalTask(ctx, &rt); err != nil {
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
