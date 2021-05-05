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

func mustString(s string, _ error) string {
	return s
}

func (lr *logRecorder) ObserveTask(task tasks.Task) error {
	duration := time.Since(task.StartedAt.Must().Time()).Milliseconds()

	if task.RetrievalTask.Exists() {
		rt := task.RetrievalTask.Must()
		lr.log.Infow("retrieval task",
			metrics.UUID, task.UUID,
			metrics.Miner, rt.Miner,
			metrics.PayloadCID, rt.PayloadCID,
			metrics.CARExport, rt.CARExport,
			metrics.Status, mustString(task.Status.AsString()),
			"duration (ms)", duration)
		return nil
	}
	if task.StorageTask.Exists() {
		st := task.StorageTask.Must()
		lr.log.Infow("storage task",
			metrics.UUID, mustString(task.UUID.AsString()),
			metrics.Miner, st.Miner,
			metrics.MaxPriceAttoFIL, st.MaxPriceAttoFIL,
			metrics.Size, st.Size,
			metrics.StartOffset, st.StartOffset,
			metrics.FastRetrieval, st.FastRetrieval,
			metrics.Verified, st.Verified,
			metrics.Status, mustString(task.Status.AsString()),
			"duration (ms)", duration)
		return nil
	}
	return fmt.Errorf("Cannot observe task: %s, both tasks are nil", task.UUID)
}
