package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/controller"
	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/controller/state"
	"github.com/filecoin-project/dealbot/controller/state/postgresdb"
	"github.com/filecoin-project/dealbot/controller/state/postgresdb/temporary"
	"github.com/filecoin-project/dealbot/metrics/testrecorder"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

const jsonTestDeals = "../devnet/sample_tasks.json"

func mustString(s string, _ error) string {
	return s
}

func TestControllerHTTPInterface(t *testing.T) {
	ctx := context.Background()
	migrator, err := state.NewMigrator("postgres")
	require.NoError(t, err)
	var dbConn state.DBConnector
	existingPGconn := postgresdb.PostgresConfig{}.String()
	dbConn = postgresdb.New(existingPGconn)
	err = dbConn.Connect()
	if err != nil {
		tempPG, err := temporary.NewTemporaryPostgres(ctx, temporary.Params{HostPort: defaultPGPort})
		require.NoError(t, err)
		dbConn = tempPG
		defer tempPG.Shutdown(ctx)
	}

	testCases := map[string]func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder){
		"list and update tasks": func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder) {
			require.NoError(t, populateTestTasksFromFile(ctx, jsonTestDeals, apiClient))

			currentTasks, err := apiClient.ListTasks(ctx)
			require.NoError(t, err)
			require.Len(t, currentTasks, 4)

			// update a task
			pt := tasks.Type.PopTask.Of("dealbot 1", tasks.InProgress)
			task, err := apiClient.PopTask(ctx, pt)
			taskUUID := mustString(task.UUID.AsString())
			require.NoError(t, err)
			require.Equal(t, *tasks.InProgress, task.Status)
			refetchTask, err := apiClient.GetTask(ctx, mustString(task.UUID.AsString()))
			require.NoError(t, err)
			require.Equal(t, *tasks.InProgress, refetchTask.Status)
			require.Equal(t, "dealbot 1", refetchTask.WorkedBy.Must().String())

			// update but from the wrong dealbot
			task, err = apiClient.UpdateTask(ctx, taskUUID, tasks.Type.UpdateTask.Of("dealbot 2", tasks.Successful, 1))
			// request fails
			require.EqualError(t, err, client.ErrRequestFailed{Code: http.StatusBadRequest}.Error())
			require.Nil(t, task)
			refetchTask, err = apiClient.GetTask(ctx, taskUUID)
			require.NoError(t, err)
			// status should not change
			require.Equal(t, *tasks.InProgress, refetchTask.Status)
			require.Equal(t, "dealbot 1", refetchTask.WorkedBy.Must().String())

			// update again
			task, err = apiClient.UpdateTask(ctx, taskUUID, tasks.Type.UpdateTask.Of("dealbot 1", tasks.Successful, 1))
			require.NoError(t, err)
			require.Equal(t, *tasks.Successful, task.Status)
			refetchTask, err = apiClient.GetTask(ctx, taskUUID)
			require.NoError(t, err)
			require.Equal(t, *tasks.Successful, refetchTask.Status)
			require.Equal(t, "dealbot 1", refetchTask.WorkedBy.Must().String())
			require.Equal(t, 1, int(refetchTask.RunCount.Int()))

			// update a different
			pt = tasks.Type.PopTask.Of("dealbot 2", tasks.Successful)
			task, err = apiClient.PopTask(ctx, pt)
			taskUUID = mustString(task.UUID.AsString())
			require.NoError(t, err)
			require.Equal(t, *tasks.Successful, task.Status)
			refetchTask, err = apiClient.GetTask(ctx, taskUUID)
			require.NoError(t, err)
			require.Equal(t, *tasks.Successful, refetchTask.Status)
			require.Equal(t, "dealbot 2", refetchTask.WorkedBy.Must().String())

			recorder.AssertExactObservedStatuses(t, mustString(currentTasks[0].UUID.AsString()), tasks.InProgress, tasks.Successful)
			recorder.AssertExactObservedStatuses(t, mustString(currentTasks[1].UUID.AsString()), tasks.Successful)
		},
		"pop a task": func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder) {
			require.NoError(t, populateTestTasksFromFile(ctx, jsonTestDeals, apiClient))

			updatedTask, err := apiClient.PopTask(ctx, tasks.Type.PopTask.Of("dealbot 1", tasks.InProgress))
			require.NoError(t, err)
			require.Equal(t, *tasks.InProgress, updatedTask.Status)
			refetchTask, err := apiClient.GetTask(ctx, mustString(updatedTask.UUID.AsString()))
			require.NoError(t, err)
			require.Equal(t, *tasks.InProgress, refetchTask.Status)
			require.Equal(t, "dealbot 1", refetchTask.WorkedBy.Must().String())

			// when no tasks are available, pop-task should return nil
			for {
				task, err := apiClient.PopTask(ctx, tasks.Type.PopTask.Of("dealbot 1", tasks.InProgress))
				require.NoError(t, err)
				if task == nil {
					break
				}
			}
		},
		"creating tasks": func(ctx context.Context, t *testing.T, apiClient *client.Client, _ *testrecorder.TestMetricsRecorder) {
			require.NoError(t, populateTestTasksFromFile(ctx, jsonTestDeals, apiClient))

			newStorageTask := tasks.Type.StorageTask.Of("t01000",
				100000000000000000, // 0.10 FIL
				2048,               // 1kb
				0,
				true,
				true,
				"")
			task, err := apiClient.CreateStorageTask(ctx, newStorageTask)
			require.NoError(t, err)
			require.Equal(t, task.StorageTask.Must(), newStorageTask)
			task, err = apiClient.GetTask(ctx, mustString(task.UUID.AsString()))
			require.NoError(t, err)
			require.Equal(t, task.StorageTask.Must(), newStorageTask)
			currentTasks, err := apiClient.ListTasks(ctx)
			require.NoError(t, err)
			require.Len(t, currentTasks, 5)

			newRetrievalTask := tasks.Type.RetrievalTask.Of(
				"f0127896",
				"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
				false,
				"",
			)
			task, err = apiClient.CreateRetrievalTask(ctx, newRetrievalTask)
			require.NoError(t, err)
			require.Equal(t, task.RetrievalTask.Must(), newRetrievalTask)
			task, err = apiClient.GetTask(ctx, mustString(task.UUID.AsString()))
			require.NoError(t, err)
			require.Equal(t, task.RetrievalTask.Must(), newRetrievalTask)
			currentTasks, err = apiClient.ListTasks(ctx)
			require.NoError(t, err)
			require.Len(t, currentTasks, 6)
		},
		"export finished tasks": func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder) {
			require.NoError(t, populateTestTasksFromFile(ctx, jsonTestDeals, apiClient))

			// dealbot1 takes a task.
			task, err := apiClient.PopTask(ctx, tasks.Type.PopTask.Of("dealbot1", tasks.InProgress))
			require.NoError(t, err)
			require.Equal(t, *tasks.InProgress, task.Status)

			// succeed task.
			task, err = apiClient.UpdateTask(ctx, task.GetUUID(), tasks.Type.UpdateTask.Of("dealbot1", tasks.Successful, 1))
			require.NoError(t, err)
			require.Equal(t, *tasks.Successful, task.Status)

			// drain the dealbot / finalize the task.
			require.NoError(t, apiClient.Drain(ctx, "dealbot1"))
			require.NoError(t, apiClient.Complete(ctx, "dealbot1"))

			// get the car. expect it to be non-empty at this point.
			carContents, closer, err := apiClient.CARExport(ctx)
			require.NoError(t, err)
			defer closer()
			require.Len(t, carContents.Header.Roots, 1)
			_, err = carContents.Next()
			require.NoError(t, err)
		},
		"reset worker tasks": func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder) {

			var resetWorkerTasks = `
	[{"Miner":"t01000","PayloadCID":"bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36","CARExport":false},
	{"Miner":"t01000","PayloadCID":"bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg","CARExport":false},
	{"Miner":"f0127896","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false},
	{"Miner":"f0127897","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false},
	{"Miner":"f0127898","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false},
	{"Miner":"f0127899","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false},
	{"Miner":"f0127900","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false}]
	`
			require.NoError(t, populateTestTasks(ctx, bytes.NewReader([]byte(resetWorkerTasks)), apiClient))

			worker := fmt.Sprintf("tester")
			otherWorker := fmt.Sprintf("other-worker")

			// pop two tasks and leave them in progress
			req := tasks.Type.PopTask.Of(worker, tasks.InProgress)
			inProgressTask1, err := apiClient.PopTask(ctx, req)
			require.NoError(t, err)
			inProgressTask2, err := apiClient.PopTask(ctx, req)
			require.NoError(t, err)

			// add some stage logs to the second task
			stage1 := tasks.Type.StageDetails.Of("Doing Stuff", "A good long while").WithLog("stuff happened")
			stage2 := tasks.Type.StageDetails.Of("Doing More Stuff", "A good long while").WithLog("more stuff happened")

			_, err = apiClient.UpdateTask(ctx, inProgressTask2.GetUUID(),
				tasks.Type.UpdateTask.OfStage(inProgressTask2.WorkedBy.Must().String(), tasks.InProgress, "", "Stage1", stage1, 1))
			require.NoError(t, err)
			_, err = apiClient.UpdateTask(ctx, inProgressTask2.GetUUID(),
				tasks.Type.UpdateTask.OfStage(inProgressTask2.WorkedBy.Must().String(), tasks.InProgress, "", "Stage2", stage2, 1))
			require.NoError(t, err)

			// pop a task and set it failed
			req = tasks.Type.PopTask.Of(worker, tasks.Failed)
			failedTask, err := apiClient.PopTask(ctx, req)
			require.NoError(t, err)

			// pop a task and set it successful
			req = tasks.Type.PopTask.Of(worker, tasks.Successful)
			successfulTask, err := apiClient.PopTask(ctx, req)
			require.NoError(t, err)

			// pop two tasks to the other worker and leave them in progress
			req = tasks.Type.PopTask.Of(otherWorker, tasks.InProgress)
			otherWorkerTask1, err := apiClient.PopTask(ctx, req)
			require.NoError(t, err)
			otherWorkerTask2, err := apiClient.PopTask(ctx, req)
			require.NoError(t, err)

			allTasks, err := apiClient.ListTasks(ctx)
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

			apiClient.ResetWorker(ctx, worker)

			// in progress task should now be available and unassigned
			inProgressTask1, err = apiClient.GetTask(ctx, inProgressTask1.GetUUID())
			require.Equal(t, *tasks.Available, inProgressTask1.Status)
			require.Equal(t, "", inProgressTask1.WorkedBy.Must().String())

			// in progress task should now be available and unassigned,
			// and stage logs should be wiped
			inProgressTask2, err = apiClient.GetTask(ctx, inProgressTask2.GetUUID())
			require.Equal(t, *tasks.Available, inProgressTask2.Status)
			require.Equal(t, "", inProgressTask2.WorkedBy.Must().String())
			require.Equal(t, "", inProgressTask2.Stage.String())
			require.False(t, inProgressTask2.CurrentStageDetails.Exists())
			require.False(t, inProgressTask2.PastStageDetails.Exists())

			// successful and failed records should not change
			successfulTask, err = apiClient.GetTask(ctx, successfulTask.GetUUID())
			require.Equal(t, *tasks.Successful, successfulTask.Status)
			require.Equal(t, worker, successfulTask.WorkedBy.Must().String())
			failedTask, err = apiClient.GetTask(ctx, failedTask.GetUUID())
			require.Equal(t, *tasks.Failed, failedTask.Status)
			require.Equal(t, worker, failedTask.WorkedBy.Must().String())

			// tasks for other worker should not change
			otherWorkerTask1, err = apiClient.GetTask(ctx, otherWorkerTask1.GetUUID())
			require.Equal(t, *tasks.InProgress, otherWorkerTask1.Status)
			require.Equal(t, otherWorker, otherWorkerTask1.WorkedBy.Must().String())
			otherWorkerTask2, err = apiClient.GetTask(ctx, otherWorkerTask2.GetUUID())
			require.Equal(t, *tasks.InProgress, otherWorkerTask2.Status)
			require.Equal(t, otherWorker, otherWorkerTask2.WorkedBy.Must().String())

			// unassigned task should not chang
			unassignedTask, err = apiClient.GetTask(ctx, unassignedTask.GetUUID())
			require.Equal(t, *tasks.Available, unassignedTask.Status)
			require.Equal(t, "", unassignedTask.WorkedBy.Must().String())

			// try assigning a task -- should reassign first newly available task
			req = tasks.Type.PopTask.Of(otherWorker, tasks.InProgress)
			newInProgressTask1, err := apiClient.PopTask(ctx, req)
			require.NoError(t, err)
			require.Equal(t, inProgressTask1.GetUUID(), newInProgressTask1.GetUUID())

			req = tasks.Type.PopTask.Of(worker, tasks.InProgress)
			newInProgressTask2, err := apiClient.PopTask(ctx, req)
			require.NoError(t, err)
			require.Equal(t, inProgressTask2.GetUUID(), newInProgressTask2.GetUUID())
		},

		"delete tasks": func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder) {

			var resetWorkerTasks = `
[{"Miner":"t01000","PayloadCID":"bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg","CARExport":false},
{"Miner":"t01000","PayloadCID":"bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36","CARExport":false,"Schedule":"0 0 * * *","ScheduleLimit":"168h"},
{"Miner":"f0127896","PayloadCID":"bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm","CARExport":false}]
`

			err := populateTestTasks(ctx, bytes.NewReader([]byte(resetWorkerTasks)), apiClient)
			require.NoError(t, err)

			worker := "testworker"
			// pop a task
			req := tasks.Type.PopTask.Of(worker, tasks.InProgress)
			inProgressTask1, err := apiClient.PopTask(ctx, req)

			allTasks, err := apiClient.ListTasks(ctx)
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

			err = apiClient.DeleteTask(ctx, unassignedTask.GetUUID())
			require.NoError(t, err)
			task, err := apiClient.GetTask(ctx, unassignedTask.GetUUID())
			require.EqualError(t, err, client.ErrRequestFailed{http.StatusNotFound}.Error())
			require.Nil(t, task)
			err = apiClient.DeleteTask(ctx, scheduledTask.GetUUID())
			require.NoError(t, err)
			task, err = apiClient.GetTask(ctx, scheduledTask.GetUUID())
			require.EqualError(t, err, client.ErrRequestFailed{http.StatusNotFound}.Error())
			require.Nil(t, task)
			err = apiClient.DeleteTask(ctx, inProgressTask1.GetUUID())
			require.EqualError(t, err, client.ErrRequestFailed{http.StatusBadRequest}.Error())
			err = apiClient.DeleteTask(ctx, "apples and oranges")
			require.EqualError(t, err, client.ErrRequestFailed{http.StatusNotFound}.Error())
		},
	}

	for testCase, run := range testCases {
		t.Run(testCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			state.WipeAndReset(dbConn, migrator)
			h := newHarness(ctx, t, dbConn, migrator)
			run(ctx, t, h.apiClient, h.recorder)

			h.Shutdown(t)
		})
	}
}

type harness struct {
	ctx        context.Context
	apiClient  *client.Client
	recorder   *testrecorder.TestMetricsRecorder
	controller *controller.Controller
	dbloc      string
	port       string
	serveErr   chan error
}

const defaultPGPort = 5435

func newHarness(ctx context.Context, t *testing.T, connector state.DBConnector, migrator state.Migrator) *harness {
	h := &harness{ctx: ctx}
	h.recorder = testrecorder.NewTestMetricsRecorder()
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	_, p, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	h.port = p
	h.apiClient = client.NewFromEndpoint("http://localhost:" + p)
	pr, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	require.NoError(t, err)

	be, err := state.NewStateDB(ctx, connector, migrator, "", pr, h.recorder)
	require.NoError(t, err)
	cc := cli.NewContext(cli.NewApp(), &flag.FlagSet{}, nil)
	h.controller, err = controller.NewWithDependencies(cc, listener, nil, h.recorder, be)

	h.serveErr = make(chan error, 1)
	go func() {
		err := h.controller.Serve()
		select {
		case <-ctx.Done():
		case h.serveErr <- err:
		}
	}()

	return h
}

func populateTestTasksFromFile(ctx context.Context, jsonTests string, apiClient *client.Client) error {
	sampleTaskFile, err := os.Open(jsonTestDeals)
	if err != nil {
		return err
	}
	defer sampleTaskFile.Close()
	return populateTestTasks(ctx, sampleTaskFile, apiClient)
}

func populateTestTasks(ctx context.Context, sampleTaskFile io.Reader, apiClient *client.Client) error {

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
			st := stp.Build().(tasks.StorageTask)
			if _, err = apiClient.CreateStorageTask(ctx, st); err != nil {
				return err
			}
		} else {
			rt := rtp.Build().(tasks.RetrievalTask)
			if _, err = apiClient.CreateRetrievalTask(ctx, rt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *harness) Shutdown(t *testing.T) {
	err := h.controller.Shutdown(h.ctx)
	require.NoError(t, err)
	select {
	case <-h.ctx.Done():
		t.Fatalf("no return from serve call")
	case err = <-h.serveErr:
		require.EqualError(t, err, http.ErrServerClosed.Error())
	}
}
