package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/zap"
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

func (lr *logRecorder) ObserveTask(task tasks.Task) (metrics.TaskObserver, error) {
	if task.RetrievalTask != nil {
		return &taskObserver{
			msg:       "retrieval task",
			startTime: time.Now(),
			log: lr.log.With(
				metrics.UUID, task.UUID,
				metrics.Miner, task.RetrievalTask.Miner,
				metrics.PayloadCID, task.RetrievalTask.PayloadCID,
				metrics.CARExport, task.RetrievalTask.CARExport),
		}, nil
	}
	if task.StorageTask != nil {
		return &taskObserver{
			msg:       "storage task",
			startTime: time.Now(),
			log: lr.log.With(
				metrics.UUID, task.UUID,
				metrics.Miner, task.StorageTask.Miner,
				metrics.MaxPriceAttoFIL, task.StorageTask.MaxPriceAttoFIL,
				metrics.Size, task.StorageTask.Size,
				metrics.StartOffset, task.StorageTask.StartOffset,
				metrics.FastRetrieval, task.StorageTask.FastRetrieval,
				metrics.Verified, task.StorageTask.Verified),
		}, nil
	}
	return nil, fmt.Errorf("Cannot observe task: %s, both tasks are nil", task.UUID)
}

type taskObserver struct {
	startTime time.Time
	msg       string
	log       *zap.SugaredLogger
}

func (to *taskObserver) RecordStatusUpdate(status tasks.Status) error {
	duration := time.Since(to.startTime).Milliseconds()
	to.log.Infow(to.msg, "status", tasks.StatusNames[status], "duration (ms)", duration)
	return nil
}
