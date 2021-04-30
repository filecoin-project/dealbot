package state

import (
	"context"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
)

// State provides an interface for presistence.
type State interface {
	AssignTask(ctx context.Context, req client.PopTaskRequest) (*tasks.Task, error)
	Get(ctx context.Context, uuid string) (*tasks.Task, error)
	GetAll(ctx context.Context) ([]*tasks.Task, error)
	Update(ctx context.Context, uuid string, req client.UpdateTaskRequest) (*tasks.Task, error)
	NewStorageTask(ctx context.Context, storageTask *tasks.StorageTask) (*tasks.Task, error)
	NewRetrievalTask(ctx context.Context, retrievalTask *tasks.RetrievalTask) (*tasks.Task, error)
}
