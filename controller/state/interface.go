package state

import (
	"context"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
)

// State provides an interface for presistence.
type State interface {
	AssignTask(ctx context.Context, req client.UpdateTaskRequest) (*tasks.AuthenticatedTask, error)
	Get(ctx context.Context, uuid string) (*tasks.AuthenticatedTask, error)
	GetAll(ctx context.Context) ([]*tasks.AuthenticatedTask, error)
	Update(ctx context.Context, uuid string, req client.UpdateTaskRequest) (*tasks.AuthenticatedTask, error)
	NewStorageTask(ctx context.Context, storageTask *tasks.StorageTask) (*tasks.AuthenticatedTask, error)
	NewRetrievalTask(ctx context.Context, retrievalTask *tasks.RetrievalTask) (*tasks.AuthenticatedTask, error)
}
