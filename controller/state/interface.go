package state

import (
	"context"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
)

type State interface {
	Get(ctx context.Context, uuid string) (*tasks.AuthenticatedTask, error)
	GetAll(ctx context.Context) ([]*tasks.AuthenticatedTask, error)
	Update(ctx context.Context, uuid string, req *client.UpdateTaskRequest, recorder metrics.MetricsRecorder) (*tasks.AuthenticatedTask, error)
	NewStorageTask(ctx context.Context, storageTask *tasks.StorageTask) (*tasks.AuthenticatedTask, error)
	NewRetrievalTask(ctx context.Context, retrievalTask *tasks.RetrievalTask) (*tasks.AuthenticatedTask, error)
}
