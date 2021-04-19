package controller

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/controller/sqlitedb"
	"github.com/filecoin-project/dealbot/metrics/testrecorder"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/stretchr/testify/require"
)

func TestControllerHTTPInterface(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	apiClient := client.NewFromEndpoint("http://localhost:3333")
	recorder := testrecorder.NewTestMetricsRecorder()
	listener, err := net.Listen("tcp", "localhost:3333")
	require.NoError(t, err)

	state, err := NewState(sqlitedb.New(testDBFile))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(testDBFile)

	c := NewWithDependencies(listener, recorder, state)
	serveErr := make(chan error, 1)
	go func() {
		err := c.Serve()
		select {
		case <-ctx.Done():
		case serveErr <- err:
		}
	}()
	currentTasks, err := apiClient.ListTasks(ctx)
	require.NoError(t, err)
	require.Len(t, currentTasks, 4)

	// update a task
	_, status, err := apiClient.UpdateTask(ctx, &client.UpdateTaskRequest{
		UUID:     currentTasks[0].UUID,
		WorkedBy: "dealbot 1",
		Status:   tasks.InProgress,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	currentTasks, err = apiClient.ListTasks(ctx)
	require.NoError(t, err)
	require.Equal(t, tasks.InProgress, currentTasks[0].Status)
	require.Equal(t, "dealbot 1", currentTasks[0].WorkedBy)

	// update but from the wrong dealbot
	_, status, err = apiClient.UpdateTask(ctx, &client.UpdateTaskRequest{
		UUID:     currentTasks[0].UUID,
		WorkedBy: "dealbot 2",
		Status:   tasks.Successful,
	})
	require.NoError(t, err)
	// request fails
	require.Equal(t, http.StatusBadRequest, status)
	currentTasks, err = apiClient.ListTasks(ctx)
	require.NoError(t, err)
	// status should not change
	require.Equal(t, tasks.InProgress, currentTasks[0].Status)
	require.Equal(t, "dealbot 1", currentTasks[0].WorkedBy)

	// update again
	_, status, err = apiClient.UpdateTask(ctx, &client.UpdateTaskRequest{
		UUID:     currentTasks[0].UUID,
		WorkedBy: "dealbot 1",
		Status:   tasks.Successful,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	currentTasks, err = apiClient.ListTasks(ctx)
	require.NoError(t, err)
	require.Equal(t, tasks.Successful, currentTasks[0].Status)
	require.Equal(t, "dealbot 1", currentTasks[0].WorkedBy)

	// update a different task
	_, status, err = apiClient.UpdateTask(ctx, &client.UpdateTaskRequest{
		UUID:     currentTasks[1].UUID,
		WorkedBy: "dealbot 2",
		Status:   tasks.Successful,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	currentTasks, err = apiClient.ListTasks(ctx)
	require.NoError(t, err)
	require.Equal(t, tasks.Successful, currentTasks[1].Status)
	require.Equal(t, "dealbot 2", currentTasks[1].WorkedBy)

	err = c.Shutdown(ctx)
	require.NoError(t, err)
	select {
	case <-ctx.Done():
		t.Fatalf("no return from serve call")
	case err = <-serveErr:
		require.EqualError(t, err, http.ErrServerClosed.Error())
	}

	recorder.AssertExactObservedStatuses(t, currentTasks[0].UUID, tasks.InProgress, tasks.Successful)
	recorder.AssertExactObservedStatuses(t, currentTasks[1].UUID, tasks.Successful)
}
