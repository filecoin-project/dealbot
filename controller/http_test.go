package controller_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/controller"
	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/metrics/testrecorder"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/require"
)

func TestControllerHTTPInterface(t *testing.T) {
	ctx := context.Background()
	testCases := map[string]struct {
		runRequests  func(ctx context.Context, t *testing.T, apiClient *client.Client)
		checkMetrics func(t *testing.T, currentTasks []*tasks.Task, recorder *testrecorder.TestMetricsRecorder)
	}{
		"updating tasks": {
			runRequests: func(ctx context.Context, t *testing.T, apiClient *client.Client) {
				currentTasks, err := apiClient.ListTasks(ctx)
				require.NoError(t, err)
				require.Len(t, currentTasks, 4)

				// update a task
				task, err := apiClient.UpdateTask(ctx, currentTasks[0].UUID, &client.UpdateTaskRequest{
					WorkedBy: "dealbot 1",
					Status:   tasks.InProgress,
				})
				require.Equal(t, tasks.InProgress, task.Status)
				require.NoError(t, err)
				currentTasks, err = apiClient.ListTasks(ctx)
				require.NoError(t, err)
				require.Equal(t, tasks.InProgress, currentTasks[0].Status)
				require.Equal(t, "dealbot 1", currentTasks[0].WorkedBy)

				// update but from the wrong dealbot
				task, err = apiClient.UpdateTask(ctx, currentTasks[0].UUID, &client.UpdateTaskRequest{
					WorkedBy: "dealbot 2",
					Status:   tasks.Successful,
				})
				// request fails
				require.EqualError(t, err, client.ErrRequestFailed{Code: http.StatusBadRequest}.Error())
				require.Nil(t, task)
				currentTasks, err = apiClient.ListTasks(ctx)
				require.NoError(t, err)
				// status should not change
				require.Equal(t, tasks.InProgress, currentTasks[0].Status)
				require.Equal(t, "dealbot 1", currentTasks[0].WorkedBy)

				// update again
				task, err = apiClient.UpdateTask(ctx, currentTasks[0].UUID, &client.UpdateTaskRequest{
					WorkedBy: "dealbot 1",
					Status:   tasks.Successful,
				})
				require.NoError(t, err)
				require.Equal(t, tasks.Successful, task.Status)
				currentTasks, err = apiClient.ListTasks(ctx)
				require.NoError(t, err)
				require.Equal(t, tasks.Successful, currentTasks[0].Status)
				require.Equal(t, "dealbot 1", currentTasks[0].WorkedBy)

				// update a different task
				task, err = apiClient.UpdateTask(ctx, currentTasks[1].UUID, &client.UpdateTaskRequest{
					WorkedBy: "dealbot 2",
					Status:   tasks.Successful,
				})
				require.NoError(t, err)
				require.Equal(t, tasks.Successful, task.Status)
				currentTasks, err = apiClient.ListTasks(ctx)
				require.NoError(t, err)
				require.Equal(t, tasks.Successful, currentTasks[1].Status)
				require.Equal(t, "dealbot 2", currentTasks[1].WorkedBy)
			},
			checkMetrics: func(t *testing.T, currentTasks []*tasks.Task, recorder *testrecorder.TestMetricsRecorder) {
				recorder.AssertExactObservedStatuses(t, currentTasks[0].UUID, tasks.InProgress, tasks.Successful)
				recorder.AssertExactObservedStatuses(t, currentTasks[1].UUID, tasks.Successful)
			},
		},
		"creating tasks": {
			runRequests: func(ctx context.Context, t *testing.T, apiClient *client.Client) {
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
		},
	}

	for testCase, data := range testCases {
		t.Run(testCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			h := newHarness(t, ctx)
			if data.runRequests != nil {
				data.runRequests(ctx, t, h.apiClient)
			}
			if data.checkMetrics != nil {
				finalTasks, err := h.apiClient.ListTasks(ctx)
				require.NoError(t, err)
				data.checkMetrics(t, finalTasks, h.recorder)
			}
			h.Shutdown(t)
		})
	}
}

type harness struct {
	ctx        context.Context
	apiClient  *client.Client
	recorder   *testrecorder.TestMetricsRecorder
	controller *controller.Controller
	serveErr   chan error
}

func newHarness(t *testing.T, ctx context.Context) *harness {
	h := &harness{ctx: ctx}
	h.apiClient = client.NewFromEndpoint("http://localhost:3333")
	h.recorder = testrecorder.NewTestMetricsRecorder()
	listener, err := net.Listen("tcp", "localhost:3333")
	require.NoError(t, err)
	pr, _, _ := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	h.controller = controller.NewWithDependencies(listener, h.recorder, pr)
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
