package metrics

import (
	"net/http"

	"github.com/filecoin-project/dealbot/tasks"
)

// MetricsRecorder abstracts the process of recording metrics for tasks
type MetricsRecorder interface {
	Handler() http.Handler
	ObserveTask(tasks.Task) error
}
