package prometheus

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// Namespace is the name space for metrics exported to prometheus
	Namespace = "dealbot"
	// StorageTasks is the subsystem to record storage task metrics
	StorageTasks = "storage_tasks"
	// RetrievalTasks is the subsystem to record retrieval tasks metrics
	RetrievalTasks = "retrieval_tasks"
	// Duration is the name of our metric -- the duration it took to get to the given status
	Duration = "duration"
	// Help is a description of what duration measures
	Help = "task duration in milliseconds to get to the specified status"
)

// Buckets are the default histogram buckets for measuring duration
// (essentially, second, minute, hour, day, and everything else)
var Buckets = []float64{
	float64(time.Second.Milliseconds()),
	float64(time.Minute.Milliseconds()),
	float64(time.Hour.Milliseconds()),
	float64(time.Hour.Milliseconds() * 24),
}

// StorageLabels are the ways we categorize storage tasks
// TODO: do we want ALL of these labels? It will mean a LOT of data
var StorageLabels = []string{metrics.UUID, metrics.Status, metrics.Miner, metrics.MaxPriceAttoFIL, metrics.Size, metrics.StartOffset, metrics.FastRetrieval, metrics.Verified}

// RetrievalLabels are the way we categorize retrieval tasks
// TODO: do we want ALL of these labels? Do PayloadCID/CARExport matter here?
var RetrievalLabels = []string{metrics.UUID, metrics.Status, metrics.Miner, metrics.PayloadCID, metrics.CARExport}

type prometheusMetricsRecorder struct {
	storageVec   *prometheus.HistogramVec
	retrievalVec *prometheus.HistogramVec
}

// NewPrometheusMetricsRecorder returns a recorder that is connected to prometheus
func NewPrometheusMetricsRecorder() metrics.MetricsRecorder {
	return &prometheusMetricsRecorder{
		storageVec: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: StorageTasks,
			Name:      Duration,
			Help:      Help,
			Buckets:   Buckets,
		}, StorageLabels),
		retrievalVec: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: Namespace,
			Subsystem: RetrievalTasks,
			Name:      Duration,
			Help:      Help,
			Buckets:   Buckets,
		}, RetrievalLabels),
	}
}

func (pmr *prometheusMetricsRecorder) Handler() http.Handler {
	return promhttp.Handler()
}

func (pmr *prometheusMetricsRecorder) ObserveTask(task tasks.Task) error {

	if task.RetrievalTask.Exists() {
		return pmr.observeRetrievalTask(task)
	}
	if task.StorageTask.Exists() {
		return pmr.observeStorageTask(task)
	}
	return fmt.Errorf("Cannot observe task: %s, both tasks are nil", task.UUID)
}

func mustString(s string, _ error) string {
	return s
}

func (pmr *prometheusMetricsRecorder) observeStorageTask(task tasks.Task) error {
	observer, err := pmr.storageVec.GetMetricWith(prometheus.Labels{
		metrics.UUID:            task.GetUUID(),
		metrics.Miner:           mustString(task.StorageTask.Must().Miner.AsString()),
		metrics.MaxPriceAttoFIL: strconv.FormatUint(uint64(task.StorageTask.Must().MaxPriceAttoFIL.Int()), 10),
		metrics.Size:            strconv.FormatUint(uint64(task.StorageTask.Must().Size.Int()), 10),
		metrics.StartOffset:     strconv.FormatUint(uint64(task.StorageTask.Must().StartOffset.Int()), 10),
		metrics.FastRetrieval:   strconv.FormatBool(task.StorageTask.Must().FastRetrieval.Bool()),
		metrics.Verified:        strconv.FormatBool(task.StorageTask.Must().Verified.Bool()),
		metrics.Status:          task.Status.String(),
	})
	if err != nil {
		return err
	}
	observer.Observe(float64(time.Since(task.StartedAt.Must().Time()).Milliseconds()))
	return nil
}

func (pmr *prometheusMetricsRecorder) observeRetrievalTask(task tasks.Task) error {
	observer, err := pmr.retrievalVec.GetMetricWith(prometheus.Labels{
		metrics.UUID:       task.GetUUID(),
		metrics.Miner:      task.RetrievalTask.Must().Miner.String(),
		metrics.PayloadCID: task.RetrievalTask.Must().PayloadCID.String(),
		metrics.CARExport:  strconv.FormatBool(task.RetrievalTask.Must().CARExport.Bool()),
		metrics.Status:     task.Status.String(),
	})
	if err != nil {
		return err
	}
	observer.Observe(float64(time.Since(task.StartedAt.Must().Time()).Milliseconds()))
	return nil
}
