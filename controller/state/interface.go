package state

import (
	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
)

type State interface {
	Get(UUID string) (*tasks.AuthenticatedTask, error)
	GetAll() ([]*tasks.AuthenticatedTask, error)
	Update(UUID string, req *client.UpdateTaskRequest, recorder metrics.MetricsRecorder) (*tasks.AuthenticatedTask, error)
	NewStorageTask(storageTask *tasks.StorageTask) (*tasks.AuthenticatedTask, error)
	NewRetrievalTask(retrievalTask *tasks.RetrievalTask) (*tasks.AuthenticatedTask, error)
}
