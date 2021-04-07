package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/pborman/uuid"
)

var State *state

type state struct {
	tasks []*tasks.Task
	mu    sync.Mutex
}

func (s *state) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.tasks)
}

func (s *state) Update(req *client.UpdateTaskRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.tasks {
		if t.UUID == req.UUID {
			if t.Status == tasks.Available {
				t.Status = req.Status
				t.WorkedBy = req.WorkedBy

				return nil
			} else {
				return errors.New("task already acquired")
			}
		}
	}

	return fmt.Errorf("cannot find task with uuid: %s", req.UUID)
}

func init() {
	State = &state{}
	State.tasks = []*tasks.Task{
		&tasks.Task{
			UUID:   uuid.New()[:8],
			Status: tasks.Available,
			RetrievalTask: &tasks.RetrievalTask{
				Miner:      "f0127896",
				PayloadCID: "bafykbzacebtvud3mqzpo3bfnq3ncayi2quhj4lpbfpqk6fid2q6oz2wkjwmsg",
				CARExport:  false,
			},
		},
		&tasks.Task{
			UUID:   uuid.New()[:8],
			Status: tasks.Available,
			RetrievalTask: &tasks.RetrievalTask{
				Miner:      "f0127896",
				PayloadCID: "bafykbzacedbytx65vf2n2daoallyvtrc52pguqvpmofgehncs6p5tk2qbg7pa",
				CARExport:  false,
			},
		},
		&tasks.Task{
			UUID:   uuid.New()[:8],
			Status: tasks.Available,
			RetrievalTask: &tasks.RetrievalTask{
				Miner:      "f0127896",
				PayloadCID: "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
				CARExport:  false,
			},
		},
		&tasks.Task{
			UUID:   uuid.New()[:8],
			Status: tasks.Available,
			StorageTask: &tasks.StorageTask{
				Miner:           "t01000",
				MaxPriceAttoFIL: 100000000000000000, // 0.10 FIL
				Size:            1024,               // 1024mb
				StartOffset:     0,
				FastRetrieval:   true,
				Verified:        false,
			},
		},
	}

}
