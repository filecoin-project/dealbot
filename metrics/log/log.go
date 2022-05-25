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

func (lr *logRecorder) ObserveTask(task *tasks.Task) error {
	duration := time.Since(task.StartedAt.Time()).Milliseconds()

	if task.RetrievalTask != nil {
		rt := task.RetrievalTask
		lr.log.Infow("retrieval task",
			metrics.UUID, task.UUID,
			metrics.Miner, rt.Miner,
			metrics.PayloadCID, rt.PayloadCID,
			metrics.CARExport, rt.CARExport,
			metrics.Status, task.Status,
			"duration (ms)", duration)
		return nil
	}
	if task.StorageTask != nil {
		st := task.StorageTask
		lr.log.Infow("storage task",
			metrics.UUID, task.UUID,
			metrics.Miner, st.Miner,
			metrics.MaxPriceAttoFIL, st.MaxPriceAttoFIL,
			metrics.Size, st.Size,
			metrics.StartOffset, st.StartOffset,
			metrics.FastRetrieval, st.FastRetrieval,
			metrics.Verified, st.Verified,
			metrics.Status, task.Status,
			"duration (ms)", duration)
		return nil
	}
	return fmt.Errorf("Cannot observe task: %s, both tasks are nil", task.UUID)
}
