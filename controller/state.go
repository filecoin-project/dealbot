package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/google/uuid"
)

var State *state

type state struct {
	tasks []*tasks.Task
	mu    sync.RWMutex
}

func (s *state) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.tasks)
}

func (s *state) Get(UUID string) (*tasks.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.tasks {
		if t.UUID == UUID {
			return t, nil
		}
	}

	return nil, fmt.Errorf("cannot find task with uuid: %s", UUID)
}

func (s *state) Update(UUID string, req *client.UpdateTaskRequest, recorder metrics.MetricsRecorder) (*tasks.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.tasks {
		if t.UUID == UUID {
			if t.Status == tasks.Available {
				t.WorkedBy = req.WorkedBy
				t.StartedAt = time.Now()
			} else {
				if t.WorkedBy != req.WorkedBy {
					return nil, errors.New("task already acquired")
				}
			}
			log.Infow("state update", "uuid", t.UUID, "status", req.Status, "worked_by", req.WorkedBy)

			t.Status = req.Status
			if err := recorder.ObserveTask(t); err != nil {
				return nil, err
			}
			return t, nil
		}
	}

	return nil, fmt.Errorf("cannot find task with uuid: %s", UUID)
}

func (s *state) NewStorageTask(storageTask *tasks.StorageTask) (*tasks.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := &tasks.Task{
		UUID:        uuid.New().String()[:8],
		Status:      tasks.Available,
		StorageTask: storageTask,
	}

	s.tasks = append(s.tasks, task)
	return task, nil
}

func (s *state) NewRetrievalTask(retrievalTask *tasks.RetrievalTask) (*tasks.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := &tasks.Task{
		UUID:          uuid.New().String()[:8],
		Status:        tasks.Available,
		RetrievalTask: retrievalTask,
	}

	s.tasks = append(s.tasks, task)
	return task, nil
}

func init() {
	State = &state{}
	State.tasks = []*tasks.Task{
		{
			UUID:   uuid.New().String()[:8],
			Status: tasks.Available,
			RetrievalTask: &tasks.RetrievalTask{
				Miner:      "t01000",
				PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
				CARExport:  false,
			},
		},
		{
			UUID:   uuid.New().String()[:8],
			Status: tasks.Available,
			RetrievalTask: &tasks.RetrievalTask{
				Miner:      "t01000",
				PayloadCID: "bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg",
				CARExport:  false,
			},
		},
		{
			UUID:   uuid.New().String()[:8],
			Status: tasks.Available,
			RetrievalTask: &tasks.RetrievalTask{
				Miner:      "f0127896",
				PayloadCID: "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
				CARExport:  false,
			},
		},
		{
			UUID:   uuid.New().String()[:8],
			Status: tasks.Available,
			StorageTask: &tasks.StorageTask{
				Miner:           "t01000",
				MaxPriceAttoFIL: 100000000000000000, // 0.10 FIL
				Size:            1024,               // 1kb
				StartOffset:     0,
				FastRetrieval:   true,
				Verified:        false,
			},
		},
	}

}
