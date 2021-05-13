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

func (lr *logRecorder) ObserveTask(task tasks.Task) error {
	duration := time.Since(task.StartedAt.Must().Time()).Milliseconds()

	if task.RetrievalTask.Exists() {
		rt := task.RetrievalTask.Must()
		lr.log.Infow("retrieval task",
			metrics.UUID, task.UUID.String(),
			metrics.Miner, rt.Miner.String(),
			metrics.PayloadCID, rt.PayloadCID.String(),
			metrics.CARExport, rt.CARExport.Bool(),
			metrics.Status, task.Status.Int(),
			"duration (ms)", duration)
		return nil
	}
	if task.StorageTask.Exists() {
		st := task.StorageTask.Must()
		lr.log.Infow("storage task",
			metrics.UUID, task.UUID.String(),
			metrics.Miner, st.Miner.String(),
			metrics.MaxPriceAttoFIL, st.MaxPriceAttoFIL.Int(),
			metrics.Size, st.Size.Int(),
			metrics.StartOffset, st.StartOffset.Int(),
			metrics.FastRetrieval, st.FastRetrieval.Bool(),
			metrics.Verified, st.Verified.Bool(),
			metrics.Status, task.Status.Int(),
			"duration (ms)", duration)
		return nil
	}
	return fmt.Errorf("Cannot observe task: %s, both tasks are nil", task.UUID)
}
