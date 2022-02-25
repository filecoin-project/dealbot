package state

import (
	"context"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/ipfs/go-cid"
)

// State provides an interface for persistence.
type State interface {
	AssignTask(ctx context.Context, req tasks.PopTask) (tasks.Task, error)
	Get(ctx context.Context, uuid string) (tasks.Task, error)
	GetAll(ctx context.Context) ([]tasks.Task, error)
	GetHead(ctx context.Context, walkback int) (tasks.RecordUpdate, error)
	Update(ctx context.Context, uuid string, req tasks.UpdateTask) (tasks.Task, error)
	NewStorageTask(ctx context.Context, storageTask tasks.StorageTask) (tasks.Task, error)
	NewRetrievalTask(ctx context.Context, retrievalTask tasks.RetrievalTask) (tasks.Task, error)
	DrainWorker(ctx context.Context, worker string) error
	UndrainWorker(ctx context.Context, worker string) error
	PublishRecordsFrom(ctx context.Context, worker string) (cid.Cid, error)
	ResetWorkerTasks(ctx context.Context, worker string) error
	Delete(ctx context.Context, uuid string) error
	Store(ctx context.Context) Store
}

type Store interface {
	Head() (cid.Cid, error)
	Has(context.Context, string) (bool, error)
	Get(context.Context, string) ([]byte, error)
	Put(ctx context.Context, key string, content []byte) error
}
