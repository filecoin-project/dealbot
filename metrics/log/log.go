package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	logging "github.com/ipfs/go-log/v2"
)

type logRecorder struct {
	log *logging.ZapEventLogger
}

// NewLogMetricsRecorder returns a metrics recorder that simply logs metrics
func NewLogMetricsRecorder(log *logging.ZapEventLogger) metrics.MetricsRecorder {
	return &logRecorder{log}
}

func (lr *logRecorder) Handler() http.Handler {
	return nil
}

func (lr *logRecorder) ObserveTask(task *tasks.AuthenticatedTask) error {
	duration := time.Since(task.StartedAt).Milliseconds()

	if task.RetrievalTask != nil {
		lr.log.Infow("retrieval task",
			metrics.UUID, task.UUID,
			metrics.Miner, task.RetrievalTask.Miner,
			metrics.PayloadCID, task.RetrievalTask.PayloadCID,
			metrics.CARExport, task.RetrievalTask.CARExport,
			metrics.Status, task.Status,
			"duration (ms)", duration)
		return nil
	}
	if task.StorageTask != nil {
		lr.log.Infow("storage task",
			metrics.UUID, task.UUID,
			metrics.Miner, task.StorageTask.Miner,
			metrics.MaxPriceAttoFIL, task.StorageTask.MaxPriceAttoFIL,
			metrics.Size, task.StorageTask.Size,
			metrics.StartOffset, task.StorageTask.StartOffset,
			metrics.FastRetrieval, task.StorageTask.FastRetrieval,
			metrics.Verified, task.StorageTask.Verified,
			metrics.Status, task.Status,
			"duration (ms)", duration)
		return nil
	}
	return fmt.Errorf("Cannot observe task: %s, both tasks are nil", task.UUID)
}
