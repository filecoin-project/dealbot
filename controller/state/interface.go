package state

import (
	"context"

	"github.com/filecoin-project/dealbot/tasks"
)

// State provides an interface for presistence.
type State interface {
	AssignTask(ctx context.Context, req tasks.PopTask) (tasks.Task, error)
	Get(ctx context.Context, uuid string) (tasks.Task, error)
	GetAll(ctx context.Context) ([]tasks.Task, error)
	GetHead(ctx context.Context) (tasks.RecordUpdate, error)
	Update(ctx context.Context, uuid string, req tasks.UpdateTask) (tasks.Task, error)
	NewStorageTask(ctx context.Context, storageTask tasks.StorageTask) (tasks.Task, error)
	NewRetrievalTask(ctx context.Context, retrievalTask tasks.RetrievalTask) (tasks.Task, error)
}
