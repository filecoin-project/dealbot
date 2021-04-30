package controller_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/controller"
	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/controller/state"
	"github.com/filecoin-project/dealbot/metrics/testrecorder"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/require"
)

const jsonTestDeals = "../devnet/sample_tasks.json"

func TestControllerHTTPInterface(t *testing.T) {
	ctx := context.Background()
	testCases := map[string]func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder){
		"list and update tasks": func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder) {
			currentTasks, err := apiClient.ListTasks(ctx)
			require.NoError(t, err)
			require.Len(t, currentTasks, 4)

			// update a task
			task, err := apiClient.PopTask(ctx, &client.PopTaskRequest{
				WorkedBy: "dealbot 1",
				Status:   tasks.InProgress,
			})
			taskUUID := task.UUID
			require.NoError(t, err)
			require.Equal(t, tasks.InProgress, task.Status)
			refetchTask, err := apiClient.GetTask(ctx, task.UUID)
			require.NoError(t, err)
			require.Equal(t, tasks.InProgress, refetchTask.Status)
			require.Equal(t, "dealbot 1", refetchTask.WorkedBy)

			// update but from the wrong dealbot
			task, err = apiClient.UpdateTask(ctx, taskUUID, &client.UpdateTaskRequest{
				WorkedBy: "dealbot 2",
				Status:   tasks.Successful,
			})
			// request fails
			require.EqualError(t, err, client.ErrRequestFailed{Code: http.StatusBadRequest}.Error())
			require.Nil(t, task)
			refetchTask, err = apiClient.GetTask(ctx, taskUUID)
			require.NoError(t, err)
			// status should not change
			require.Equal(t, tasks.InProgress, refetchTask.Status)
			require.Equal(t, "dealbot 1", refetchTask.WorkedBy)

			// update again
			task, err = apiClient.UpdateTask(ctx, taskUUID, &client.UpdateTaskRequest{
				WorkedBy: "dealbot 1",
				Status:   tasks.Successful,
			})
			require.NoError(t, err)
			require.Equal(t, tasks.Successful, task.Status)
			refetchTask, err = apiClient.GetTask(ctx, taskUUID)
			require.NoError(t, err)
			require.Equal(t, tasks.Successful, refetchTask.Status)
			require.Equal(t, "dealbot 1", refetchTask.WorkedBy)

			// update a different
			task, err = apiClient.PopTask(ctx, &client.PopTaskRequest{
				WorkedBy: "dealbot 2",
				Status:   tasks.Successful,
			})
			taskUUID = task.UUID
			require.NoError(t, err)
			require.Equal(t, tasks.Successful, task.Status)
			refetchTask, err = apiClient.GetTask(ctx, taskUUID)
			require.NoError(t, err)
			require.Equal(t, tasks.Successful, refetchTask.Status)
			require.Equal(t, "dealbot 2", refetchTask.WorkedBy)

			recorder.AssertExactObservedStatuses(t, currentTasks[0].UUID, tasks.InProgress, tasks.Successful)
			recorder.AssertExactObservedStatuses(t, currentTasks[1].UUID, tasks.Successful)
		},
		"pop a task": func(ctx context.Context, t *testing.T, apiClient *client.Client, recorder *testrecorder.TestMetricsRecorder) {
			updatedTask, err := apiClient.PopTask(ctx, &client.PopTaskRequest{
				WorkedBy: "dealbot 1",
				Status:   tasks.InProgress,
			})
			require.NoError(t, err)
			require.Equal(t, tasks.InProgress, updatedTask.Status)
			refetchTask, err := apiClient.GetTask(ctx, updatedTask.UUID)
			require.NoError(t, err)
			require.Equal(t, tasks.InProgress, refetchTask.Status)
			require.Equal(t, "dealbot 1", refetchTask.WorkedBy)

			// when no tasks are available, pop-task should return nil
			for {
				task, err := apiClient.PopTask(ctx, &client.PopTaskRequest{
					WorkedBy: "dealbot 1",
					Status:   tasks.InProgress,
				})
				require.NoError(t, err)
				if task == nil {
					break
				}
			}
		},
		"creating tasks": func(ctx context.Context, t *testing.T, apiClient *client.Client, _ *testrecorder.TestMetricsRecorder) {
			newStorageTask := &tasks.StorageTask{
				Miner:           "t01000",
				MaxPriceAttoFIL: 100000000000000000, // 0.10 FIL
				Size:            2048,               // 1kb
				StartOffset:     0,
				FastRetrieval:   true,
				Verified:        true,
			}
			task, err := apiClient.CreateStorageTask(ctx, newStorageTask)
			require.NoError(t, err)
			require.Equal(t, task.StorageTask, newStorageTask)
			task, err = apiClient.GetTask(ctx, task.UUID)
			require.NoError(t, err)
			require.Equal(t, task.StorageTask, newStorageTask)
			currentTasks, err := apiClient.ListTasks(ctx)
			require.NoError(t, err)
			require.Len(t, currentTasks, 5)

			newRetrievalTask := &tasks.RetrievalTask{
				Miner:      "f0127896",
				PayloadCID: "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
				CARExport:  false,
			}
			task, err = apiClient.CreateRetrievalTask(ctx, newRetrievalTask)
			require.NoError(t, err)
			require.Equal(t, task.RetrievalTask, newRetrievalTask)
			task, err = apiClient.GetTask(ctx, task.UUID)
			require.NoError(t, err)
			require.Equal(t, task.RetrievalTask, newRetrievalTask)
			currentTasks, err = apiClient.ListTasks(ctx)
			require.NoError(t, err)
			require.Len(t, currentTasks, 6)
		},
	}

	for testCase, run := range testCases {
		t.Run(testCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			h := newHarness(ctx, t)
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

func newHarness(ctx context.Context, t *testing.T) *harness {
	h := &harness{ctx: ctx}
	h.recorder = testrecorder.NewTestMetricsRecorder()
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	_, p, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	h.port = p
	h.apiClient = client.NewFromEndpoint("http://localhost:" + p)
	pr, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	h.dbloc, err = ioutil.TempDir("", "dealbot_test_*")
	require.NoError(t, err)
	be, err := state.NewStateDB(ctx, "sqlite", h.dbloc+"/tmp.sqlite", pr, h.recorder)
	require.NoError(t, err)
	h.controller = controller.NewWithDependencies(listener, h.recorder, be)

	h.serveErr = make(chan error, 1)
	go func() {
		err := h.controller.Serve()
		select {
		case <-ctx.Done():
		case h.serveErr <- err:
		}
	}()

	// populate test tasks
	require.NoError(t, populateTestTasks(ctx, jsonTestDeals, h.apiClient))

	return h
}

func populateTestTasks(ctx context.Context, jsonTests string, apiClient *client.Client) error {
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
			if _, err = apiClient.CreateStorageTask(ctx, &st); err != nil {
				return err
			}
		} else {
			if _, err = apiClient.CreateRetrievalTask(ctx, &rt); err != nil {
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
	if _, err := os.Stat(h.dbloc); !os.IsNotExist(err) {
		os.RemoveAll(h.dbloc)
	}
}
