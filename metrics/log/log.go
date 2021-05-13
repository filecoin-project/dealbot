package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
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

func simpleGoValue(node ipld.Node) interface{} {
	var val interface{}
	var err error
	switch kind := node.Kind(); kind {
	case ipld.Kind_String:
		val, err = node.AsString()
	case ipld.Kind_Int:
		val, err = node.AsInt()
	case ipld.Kind_Bool:
		val, err = node.AsBool()
	default:
		panic(kind.String())
	}
	if err != nil {
		panic(err)
	}
	return val
}

func (lr *logRecorder) ObserveTask(task tasks.Task) error {
	duration := time.Since(task.StartedAt.Must().Time()).Milliseconds()

	if task.RetrievalTask.Exists() {
		rt := task.RetrievalTask.Must()
		lr.log.Infow("retrieval task",
			metrics.UUID, simpleGoValue(&task.UUID),
			metrics.Miner, simpleGoValue(&rt.Miner),
			metrics.PayloadCID, simpleGoValue(&rt.PayloadCID),
			metrics.CARExport, simpleGoValue(&rt.CARExport),
			metrics.Status, simpleGoValue(&task.Status),
			"duration (ms)", duration)
		return nil
	}
	if task.StorageTask.Exists() {
		st := task.StorageTask.Must()
		lr.log.Infow("storage task",
			metrics.UUID, simpleGoValue(&task.UUID),
			metrics.Miner, simpleGoValue(&st.Miner),
			metrics.MaxPriceAttoFIL, simpleGoValue(&st.MaxPriceAttoFIL),
			metrics.Size, simpleGoValue(&st.Size),
			metrics.StartOffset, simpleGoValue(&st.StartOffset),
			metrics.FastRetrieval, simpleGoValue(&st.FastRetrieval),
			metrics.Verified, simpleGoValue(&st.Verified),
			metrics.Status, simpleGoValue(&task.Status),
			"duration (ms)", duration)
		return nil
	}
	return fmt.Errorf("Cannot observe task: %s, both tasks are nil", task.UUID)
}
