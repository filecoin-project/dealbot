package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/controller/publisher"
	"github.com/filecoin-project/dealbot/controller/state/postgresdb"
	"github.com/filecoin-project/dealbot/controller/state/postgresdb/temporary"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/go-legs"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-ds-sql/postgres"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/storage"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	pando "github.com/kenlabs/pando/pkg/types/schema"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jsonTestDeals = "../../devnet/sample_tasks.json"

var dbConn DBConnector
var migrator Migrator

func TestMain(m *testing.M) {
	var err error
	migrator, err = NewMigrator("postgres")
	if err != nil {
		fmt.Println("could not setup migrator")
		os.Exit(-1)
	}
	existingPGconn := postgresdb.PostgresConfig{}.String()
	dbConn = postgresdb.New(existingPGconn)
	err = dbConn.Connect()
	if err == nil {
		os.Exit(m.Run())
	}

	tempPG, err := temporary.NewTemporaryPostgres(context.Background(), temporary.Params{HostPort: defaultPGPort})
	if err != nil {
		fmt.Println("unable to establish postgres connection either on system or docker")
		os.Exit(-1)
	}
	dbConn = tempPG
	res := m.Run()
	tempPG.Shutdown(context.Background())
	os.Exit(res)
}

func TestLoadTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	withState(ctx, t, func(state *stateDB) {

		count, err := state.countTasks(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		rt := tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, "")
		tsk := tasks.Type.Task.New(rt, nil)
		err = state.saveTask(ctx, tsk, "", "")
		require.NoError(t, err)

		oldCount := count
		count, err = state.countTasks(ctx)
		require.NoError(t, err)
		require.Equal(t, oldCount+1, count)
	})
}

func TestAssignTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withState(ctx, t, func(state *stateDB) {

		err := populateTestTasksFromFile(ctx, jsonTestDeals, state)
		require.NoError(t, err)

		rt := tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, "")
		task := tasks.Type.Task.New(rt, nil)
		err = state.saveTask(ctx, task, "", "")
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

		var uuid string
		for uuid = range seen {
			break
		}
		task, err = state.Get(ctx, uuid)
		require.NoError(t, err)
	})
}

func TestAssignConcurrentTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withState(ctx, t, func(state *stateDB) {

		err := populateTestTasksFromFile(ctx, jsonTestDeals, state)
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
	})
}

func TestAssignTaskWithTag(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	withState(ctx, t, func(state *stateDB) {

		//err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
		//require.NoError(t, err)

		tasktag := "testtag"
		rt := tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, tasktag)
		task := tasks.Type.Task.New(rt, nil)
		err := state.saveTask(ctx, task, tasktag, "")
		require.NoError(t, err)

		tasktag = "sometag"
		rt = tasks.Type.RetrievalTask.Of("f0127896", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, tasktag)
		task = tasks.Type.Task.New(rt, nil)
		err = state.saveTask(ctx, task, tasktag, "")
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
		err = state.saveTask(ctx, task, "", "")
		require.NoError(t, err)

		// Should get untagged task
		req = tasks.Type.PopTask.Of(worker, tasks.InProgress, "foo", "bar")
		task, err = state.AssignTask(ctx, req)
		require.NotNil(t, task, "Did not get untagged task")
	})
}
func TestAssignConcurrentTaskWithTag(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withState(ctx, t, func(state *stateDB) {
		taskCount := 4

		for i := 0; i < taskCount; i++ {
			tasktag := "testtag"
			rt := tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, tasktag)
			task := tasks.Type.Task.New(rt, nil)
			err := state.saveTask(ctx, task, tasktag, "")
			require.NoError(t, err)

			tasktag = "sometag"
			rt = tasks.Type.RetrievalTask.Of("f0127896", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false, tasktag)
			task = tasks.Type.Task.New(rt, nil)
			err = state.saveTask(ctx, task, tasktag, "")
			require.NoError(t, err)

		}

		release := make(chan struct{})
		assigned := make([]tasks.Task, taskCount)
		errChan := make(chan error)
		t.Log("concurrently assigning", taskCount, "tasks")
		for i := 0; i < taskCount; i++ {
			go func(n int) {
				worker := fmt.Sprintf("worker-%d", n)
				<-release
				req := tasks.Type.PopTask.Of(worker, tasks.InProgress, "testtag")

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
			err := <-errChan
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
	})
}

func TestUpdateTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withState(ctx, t, func(state *stateDB) {

		err := populateTestTasksFromFile(ctx, jsonTestDeals, state)
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

		testCases := []struct {
			name                 string
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
			{
				name:              "attempting to work on unassigned task",
				uuid:              unassignedTask.GetUUID(),
				updateTaskRequest: tasks.Type.UpdateTask.Of("tester", tasks.InProgress, 1),
				expectedError:     ErrNotAssigned,
			},
			{
				name:              "attempting to work on task with another worker",
				uuid:              inProgressTasks[0].GetUUID(),
				updateTaskRequest: tasks.Type.UpdateTask.Of("tester 2", tasks.Successful, 1),
				expectedError:     ErrWrongWorker,
			},
			{
				name:              "update task status",
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
			{
				name:                 "update stage",
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
			{
				name:                 "update stage data within stage",
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
			{
				name:                 "update error message",
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

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				task, err := state.Update(ctx, tc.uuid, tc.updateTaskRequest)
				if tc.expectedError != nil {
					require.Nil(t, task)
					require.EqualError(t, err, tc.expectedError.Error())
				} else {
					require.NoError(t, err)
					require.Equal(t, *tc.expectedStatus, task.Status)
					if tc.expectedErrorMessage == "" {
						require.False(t, task.ErrorMessage.Exists())
					} else {
						require.Equal(t, tc.expectedErrorMessage, task.ErrorMessage.Must().String())
					}
					require.Equal(t, tc.expectedStage, task.Stage.String())
					if tc.expectedStageDetails == nil {
						require.Equal(t, task.CurrentStageDetails.Exists(), false)
					} else {
						require.Equal(t, tc.expectedStageDetails, task.CurrentStageDetails.Must())
					}
					taskEvents, err := state.TaskHistory(ctx, tc.uuid)
					require.NoError(t, err)
					require.Equal(t, len(tc.expectedTaskHistory), len(taskEvents))

					history := make([]statusHistory, len(taskEvents))
					for i, te := range taskEvents {
						history[i] = statusHistory{te.Status, te.Stage, te.Run}
					}
					require.Equal(t, tc.expectedTaskHistory, history)
					require.Equal(t, tc.expectedStage, taskEvents[len(taskEvents)-1].Stage)
					require.Equal(t, tc.expectedRun, taskEvents[len(taskEvents)-1].Run)
				}
			})
		}
	})
}

func TestResetWorkerTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withState(ctx, t, func(state *stateDB) {

		var resetWorkerTasks = `
[{"Miner":"t01000","PayloadCID":"bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36","CARExport":false},
{"Miner":"t01000","PayloadCID":"bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg","CARExport":false},
{"Miner":"f0127896","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false},
{"Miner":"f0127897","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false},
{"Miner":"f0127898","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false},
{"Miner":"f0127899","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false},
{"Miner":"f0127900","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false}]
`
		err := populateTestTasks(ctx, bytes.NewReader([]byte(resetWorkerTasks)), state)
		require.NoError(t, err)

		worker := fmt.Sprintf("tester")
		otherWorker := fmt.Sprintf("other-worker")

		// pop two tasks and leave them in progress
		req := tasks.Type.PopTask.Of(worker, tasks.InProgress)
		inProgressTask1, err := state.AssignTask(ctx, req)
		require.NoError(t, err)
		inProgressTask2, err := state.AssignTask(ctx, req)
		require.NoError(t, err)

		// add some stage logs to the second task
		stage1 := tasks.Type.StageDetails.Of("Doing Stuff", "A good long while").WithLog("stuff happened")
		stage2 := tasks.Type.StageDetails.Of("Doing More Stuff", "A good long while").WithLog("more stuff happened")

		_, err = state.Update(ctx, inProgressTask2.GetUUID(),
			tasks.Type.UpdateTask.OfStage(inProgressTask2.WorkedBy.Must().String(), tasks.InProgress, "", "Stage1", stage1, 1))
		require.NoError(t, err)
		_, err = state.Update(ctx, inProgressTask2.GetUUID(),
			tasks.Type.UpdateTask.OfStage(inProgressTask2.WorkedBy.Must().String(), tasks.InProgress, "", "Stage2", stage2, 1))
		require.NoError(t, err)

		// pop a task and set it failed
		req = tasks.Type.PopTask.Of(worker, tasks.Failed)
		failedTask, err := state.AssignTask(ctx, req)
		require.NoError(t, err)
		failedTask, err = state.Update(ctx, failedTask.GetUUID(), tasks.Type.UpdateTask.Of(inProgressTask2.WorkedBy.Must().String(), tasks.Failed, 1))
		require.NoError(t, err)

		// pop a task to in progress, then set it successful
		req = tasks.Type.PopTask.Of(worker, tasks.InProgress)
		successfulTask, err := state.AssignTask(ctx, req)
		require.NoError(t, err)
		successfulTask, err = state.Update(ctx, successfulTask.GetUUID(), tasks.Type.UpdateTask.Of(inProgressTask2.WorkedBy.Must().String(), tasks.Successful, 1))
		require.NoError(t, err)

		// pop two tasks to the other worker and leave them in progress
		req = tasks.Type.PopTask.Of(otherWorker, tasks.InProgress)
		otherWorkerTask1, err := state.AssignTask(ctx, req)
		require.NoError(t, err)
		otherWorkerTask2, err := state.AssignTask(ctx, req)
		require.NoError(t, err)

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

		state.ResetWorkerTasks(ctx, worker)

		// in progress task should now be available and unassigned
		inProgressTask1, err = state.Get(ctx, inProgressTask1.GetUUID())
		require.Equal(t, *tasks.Available, inProgressTask1.Status)
		require.Equal(t, "", inProgressTask1.WorkedBy.Must().String())

		// in progress task should now be available and unassigned,
		// and stage logs should be wiped
		inProgressTask2, err = state.Get(ctx, inProgressTask2.GetUUID())
		require.Equal(t, *tasks.Available, inProgressTask2.Status)
		require.Equal(t, "", inProgressTask2.WorkedBy.Must().String())
		require.Equal(t, "", inProgressTask2.Stage.String())
		require.False(t, inProgressTask2.CurrentStageDetails.Exists())
		require.False(t, inProgressTask2.PastStageDetails.Exists())

		// successful and failed records should not change
		successfulTask, err = state.Get(ctx, successfulTask.GetUUID())
		require.Equal(t, *tasks.Successful, successfulTask.Status)
		require.Equal(t, worker, successfulTask.WorkedBy.Must().String())
		failedTask, err = state.Get(ctx, failedTask.GetUUID())
		require.Equal(t, *tasks.Failed, failedTask.Status)
		require.Equal(t, worker, failedTask.WorkedBy.Must().String())

		// tasks for other worker should not change
		otherWorkerTask1, err = state.Get(ctx, otherWorkerTask1.GetUUID())
		require.Equal(t, *tasks.InProgress, otherWorkerTask1.Status)
		require.Equal(t, otherWorker, otherWorkerTask1.WorkedBy.Must().String())
		otherWorkerTask2, err = state.Get(ctx, otherWorkerTask2.GetUUID())
		require.Equal(t, *tasks.InProgress, otherWorkerTask2.Status)
		require.Equal(t, otherWorker, otherWorkerTask2.WorkedBy.Must().String())

		// unassigned task should not chang
		unassignedTask, err = state.Get(ctx, unassignedTask.GetUUID())
		require.Equal(t, *tasks.Available, unassignedTask.Status)
		require.Equal(t, "", unassignedTask.WorkedBy.Must().String())

		// try assigning a task -- should reassign first newly available task
		req = tasks.Type.PopTask.Of(otherWorker, tasks.InProgress)
		newInProgressTask1, err := state.AssignTask(ctx, req)
		require.NoError(t, err)
		require.Equal(t, inProgressTask1.GetUUID(), newInProgressTask1.GetUUID())

		req = tasks.Type.PopTask.Of(worker, tasks.InProgress)
		newInProgressTask2, err := state.AssignTask(ctx, req)
		require.NoError(t, err)
		require.Equal(t, inProgressTask2.GetUUID(), newInProgressTask2.GetUUID())

	})
}

func TestComplete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withState(ctx, t, func(state *stateDB) {
		err := populateTestTasksFromFile(ctx, jsonTestDeals, state)
		require.NoError(t, err)
		// dealbot1 takes a task.
		task, err := state.AssignTask(ctx, tasks.Type.PopTask.Of("dealbot1", tasks.InProgress))
		require.NoError(t, err)
		require.Equal(t, *tasks.InProgress, task.Status)

		// succeed task.
		task, err = state.Update(ctx, task.GetUUID(), tasks.Type.UpdateTask.Of("dealbot1", tasks.Successful, 1))
		require.NoError(t, err)
		require.Equal(t, *tasks.Successful, task.Status)

		require.NoError(t, err)
		// drain the dealbot / finalize the task.
		require.NoError(t, state.DrainWorker(ctx, "dealbot1"))
		uc, err := state.PublishRecordsFrom(ctx, "dealbot1")
		require.NoError(t, err)
		require.NotEqual(t, cid.Undef, uc)

		nextHead, err := state.GetHead(ctx, 0)
		require.NoError(t, err)
		require.True(t, nextHead.Previous.IsNull())
		require.Equal(t, int64(1), nextHead.Records.Length())
		iter := nextHead.Records.Iterator()
		_, record := iter.Next()
		store := state.Store(ctx)
		blk, err := store.Get(context.TODO(), record.Record.Link().(cidlink.Link).Cid.String())
		require.NoError(t, err)
		tskBuilder := tasks.Type.FinishedTask.NewBuilder()
		require.NoError(t, dagjson.Decode(tskBuilder, bytes.NewReader(blk)))
		tsk := tskBuilder.Build().(tasks.FinishedTask)
		require.Equal(t, task.RetrievalTask.Must().PayloadCID.String(), tsk.PayloadCID.Must().String())
	})
}

func TestCompleteWithPublish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withState(ctx, t, func(state *stateDB) {
		err := populateTestTasksFromFile(ctx, jsonTestDeals, state)
		require.NoError(t, err)
		// dealbot1 takes a task.
		task, err := state.AssignTask(ctx, tasks.Type.PopTask.Of("dealbot1", tasks.InProgress))
		require.NoError(t, err)
		require.Equal(t, *tasks.InProgress, task.Status)

		// succeed task.
		task, err = state.Update(ctx, task.GetUUID(), tasks.Type.UpdateTask.Of("dealbot1", tasks.Successful, 1))
		require.NoError(t, err)
		require.Equal(t, *tasks.Successful, task.Status)

		require.NoError(t, err)
		// drain the dealbot / finalize the task.
		require.NoError(t, state.DrainWorker(ctx, "dealbot1"))
		uc, err := state.PublishRecordsFrom(ctx, "dealbot1")
		require.NoError(t, err)
		require.NotEqual(t, cid.Undef, uc)

		subhost, err := libp2p.New()
		require.NoError(t, err)

		pubhost, err := libp2p.New()
		require.NoError(t, err)

		queries := postgres.NewQueries("legs_data")
		ds := NewSqlDatastore(state.dbconn, queries)
		pub, err := publisher.NewPandoPublisher(
			ds,
			state.Store(ctx),
			publisher.WithHost(pubhost),
			publisher.WithBootstrapPeers(subhost.Peerstore().PeerInfo(subhost.ID())))
		require.NoError(t, err)

		err = pub.Start(ctx)
		require.NoError(t, err)
		defer pub.Shutdown(ctx)

		err = pub.Publish(ctx, uc)
		require.NoError(t, err)

		subLS := cidlink.DefaultLinkSystem()
		subStore := &memstore.Store{}
		subLS.SetReadStorage(subStore)
		subLS.SetWriteStorage(subStore)

		sub, err := legs.NewSubscriber(subhost, dssync.MutexWrap(datastore.NewMapDatastore()), subLS, "/pando/v0.0.1", nil)
		require.NoError(t, err)

		// sync latest by specifying cid.Undef to fetch the head on the fly.
		got1stLatest, err := sub.Sync(ctx, pubhost.ID(), cid.Undef, nil, pubhost.Addrs()[0])
		require.NoError(t, err)

		// Assert that synced data is a valid pando metadata with no previous link and get the payload as CID
		got1stPayloadCid := requirePandoMetadataPayloadAsCid(t, subStore, got1stLatest, pubhost.ID(), cid.Undef)

		// Assert that the decoded metadata payload as CID is retrievable.
		got1stSyncCid, err := sub.Sync(ctx, pubhost.ID(), got1stPayloadCid, nil, pubhost.Addrs()[0])
		require.NoError(t, err)
		require.Equal(t, got1stSyncCid, got1stPayloadCid)

		// Make another record and publish it to assert previous ID is set properly in pando metadata.
		uc2, err := state.PublishRecordsFrom(ctx, "dealbot1")
		require.NoError(t, err)
		require.NotEqual(t, cid.Undef, uc2)
		require.NotEqual(t, uc, uc2)

		err = pub.Publish(ctx, uc2)
		require.NoError(t, err)

		// sync latest by specifying cid.Undef to fetch the head on the fly and assert it's different from previous latest.
		got2ndLatest, err := sub.Sync(ctx, pubhost.ID(), cid.Undef, nil, pubhost.Addrs()[0])
		require.NoError(t, err)
		require.NotEqual(t, got1stLatest, got2ndLatest)

		// Assert the synced data decodes as pando metadata with the expected publisher ID and previous link.
		got2ndPayloadCid := requirePandoMetadataPayloadAsCid(t, subStore, got2ndLatest, pubhost.ID(), got1stLatest)

		// Explicitly sync the payload to assert it is also retrievable.
		got2ndSyncCid, err := sub.Sync(ctx, pubhost.ID(), got2ndPayloadCid, nil, pubhost.Addrs()[0])
		require.NoError(t, err)
		require.Equal(t, got2ndSyncCid, got2ndPayloadCid)
	})
}

func requirePandoMetadataPayloadAsCid(t *testing.T, store storage.ReadableStorage, key cid.Cid, wantProvId peer.ID, wantPrev cid.Cid) cid.Cid {
	ctx := context.Background()
	gotBytes, err := store.Get(ctx, key.KeyString())
	require.NoError(t, err)

	// Assert the synced data decodes as a valid pando metadata.
	nb := pando.MetadataPrototype.NewBuilder()
	err = dagjson.Decode(nb, bytes.NewBuffer(gotBytes))
	require.NoError(t, err)
	gotMetadata, err := pando.UnwrapMetadata(nb.Build())
	require.NoError(t, err)

	// Assert that provider ID is as expected
	require.Equal(t, wantProvId.String(), gotMetadata.Provider)

	// Assert that the metadata payload is a valid CID.
	payloadAsLink, err := gotMetadata.Payload.AsLink()
	require.NoError(t, err)
	require.NotNil(t, payloadAsLink)

	// Assert signature validity
	sigPeerID, err := pando.VerifyMetadata(gotMetadata)
	require.NoError(t, err)
	require.Equal(t, wantProvId, sigPeerID)

	// Assert expected previous link
	hasPrev := gotMetadata.PreviousID == nil
	if wantPrev == cid.Undef {
		require.True(t, hasPrev)
	} else {
		require.False(t, hasPrev)
		gotPrev := *gotMetadata.PreviousID
		require.Equal(t, wantPrev, gotPrev.(cidlink.Link).Cid)
	}
	return payloadAsLink.(cidlink.Link).Cid
}

func TestDelete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withState(ctx, t, func(state *stateDB) {

		var resetWorkerTasks = `
[{"Miner":"t01000","PayloadCID":"bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg","CARExport":false},
{"Miner":"t01000","PayloadCID":"bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36","CARExport":false,"Schedule":"0 0 * * *","ScheduleLimit":"168h"},
{"Miner":"f0127896","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false}]
`

		err := populateTestTasks(ctx, bytes.NewReader([]byte(resetWorkerTasks)), state)
		require.NoError(t, err)

		worker := "testworker"
		// pop a task
		req := tasks.Type.PopTask.Of(worker, tasks.InProgress)
		inProgressTask1, err := state.AssignTask(ctx, req)

		allTasks, err := state.GetAll(ctx)
		require.NoError(t, err)

		var unassignedTask tasks.Task
		for _, task := range allTasks {
			if task.Status == *tasks.Available && !task.HasSchedule() {
				unassignedTask = task
				break
			}
		}
		require.NotNil(t, unassignedTask)

		var scheduledTask tasks.Task
		for _, task := range allTasks {
			if task.HasSchedule() {
				scheduledTask = task
				break
			}
		}
		require.NotNil(t, scheduledTask)

		testCases := map[string]struct {
			uuid        string
			expectedErr error
		}{
			"delete unassigned task": {
				uuid: unassignedTask.GetUUID(),
			},
			"delete scheduled task": {
				uuid: scheduledTask.GetUUID(),
			},
			"delete in progress task": {
				uuid:        inProgressTask1.GetUUID(),
				expectedErr: ErrNoDeleteInProgressTasks,
			},
			"delete unknown tasks": {
				uuid:        "alate to ate apples and bananaes",
				expectedErr: ErrTaskNotFound,
			},
		}

		for testCase, data := range testCases {
			t.Run(testCase, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				err := state.Delete(ctx, data.uuid)
				if data.expectedErr != nil {
					require.EqualError(t, err, data.expectedErr.Error())
				} else {
					require.NoError(t, err)
					task, err := state.Get(ctx, data.uuid)
					require.NoError(t, err)
					require.Nil(t, task)
					taskEvents, err := state.TaskHistory(ctx, data.uuid)
					require.NoError(t, err)
					require.Len(t, taskEvents, 0)
				}
			})
		}
	})
}

func populateTestTasksFromFile(ctx context.Context, jsonTests string, state State) error {
	sampleTaskFile, err := os.Open(jsonTestDeals)
	if err != nil {
		return err
	}
	defer sampleTaskFile.Close()
	return populateTestTasks(ctx, sampleTaskFile, state)
}

func populateTestTasks(ctx context.Context, stream io.Reader, state State) error {
	sampleTasks, err := ioutil.ReadAll(stream)
	if err != nil {
		return err
	}
	byTask := make([]json.RawMessage, 0)
	if err = json.Unmarshal(sampleTasks, &byTask); err != nil {
		return err
	}
	for _, task := range byTask {
		rtp := tasks.Type.RetrievalTask.NewBuilder()
		if err = dagjson.Decode(rtp, bytes.NewBuffer(task)); err != nil {
			stp := tasks.Type.StorageTask.NewBuilder()
			if err = dagjson.Decode(stp, bytes.NewBuffer(task)); err != nil {
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

const defaultPGPort = 5434

func withState(ctx context.Context, t *testing.T, fn func(*stateDB)) {

	key, err := makeKey()
	require.NoError(t, err)

	err = WipeAndReset(dbConn, migrator)
	require.NoError(t, err)
	stateInterface, err := NewStateDB(ctx, dbConn, migrator, "", key, nil)
	require.NoError(t, err)
	state, ok := stateInterface.(*stateDB)
	require.True(t, ok, "returned wrong type")
	fn(state)
}
