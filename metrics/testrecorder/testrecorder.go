package testrecorder

import (
	"net/http"
	"testing"

	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/stretchr/testify/assert"
)

type taskStatuses struct {
	statuses []tasks.Status
}

// TestMetricsRecorder recorder is a metrics recorder that allows you to assert expected behavior for the metrics library
type TestMetricsRecorder struct {
	tasks map[string]*taskStatuses
}

// NewTestMetricsRecorder constructs a new test metrics recorder
func NewTestMetricsRecorder() *TestMetricsRecorder {
	return &TestMetricsRecorder{
		tasks: make(map[string]*taskStatuses),
	}
}

func (tr *TestMetricsRecorder) Handler() http.Handler {
	return nil
}

func (tr *TestMetricsRecorder) ObserveTask(task tasks.Task) (metrics.TaskObserver, error) {
	existing, ok := tr.tasks[task.UUID]
	if ok {
		return existing, nil
	}
	ts := &taskStatuses{statuses: []tasks.Status{task.Status}}
	tr.tasks[task.UUID] = ts
	return ts, nil
}

// AssertObservedStatuses asserts that the given statuses were among those observed for the given task
func (tr *TestMetricsRecorder) AssertObservedStatuses(t *testing.T, uuid string, expectedStatuses ...tasks.Status) {
	ts, ok := tr.tasks[uuid]
	assert.True(t, ok, "no statuses for tasks")
	for _, status := range expectedStatuses {
		assert.Contains(t, ts.statuses, status)
	}
}

// AssertExactObservedStatuses asserts the the expected statuses we the exact statuses observed for the given task,
// in order, with no other statuses observed
func (tr *TestMetricsRecorder) AssertExactObservedStatuses(t *testing.T, uuid string, expectedStatuses ...tasks.Status) {
	ts, ok := tr.tasks[uuid]
	assert.True(t, ok, "no statuses for tasks")
	assert.Equal(t, expectedStatuses, ts.statuses)
}

func (ts *taskStatuses) RecordStatusUpdate(status tasks.Status) error {
	ts.statuses = append(ts.statuses, status)
	return nil
}
