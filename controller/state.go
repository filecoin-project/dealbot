package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/google/uuid"
)

type state struct {
	db *sql.DB
}

func NewState(ctx context.Context, db DBConnector) (*state, error) {
	err := db.Connect()
	if err != nil {
		return nil, err
	}
	sqldb := db.SqlDB()

	_, err = sqldb.ExecContext(ctx, createTasksTable)
	if err != nil {
		return nil, err
	}

	s := &state{
		db: sqldb,
	}

	count, err := s.CountTasks(ctx)
	if err != nil {
		return nil, err
	}

	if count == 0 {
		err = s.createInitialTasks(ctx)
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *state) CountTasks(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, countTasksSql).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *state) MarshalJSON() ([]byte, error) {
	rows, err := s.db.Query(getAllTasks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var storedTasks []*tasks.Task
	for rows.Next() {
		var uid, jsonTask string
		if err = rows.Scan(&uid, &jsonTask); err != nil {
			return nil, err
		}

		var task tasks.Task
		if err = json.Unmarshal([]byte(jsonTask), &task); err != nil {
			return nil, err
		}

		storedTasks = append(storedTasks, &task)
	}
	return json.Marshal(storedTasks)
}

func (s *state) Update(ctx context.Context, req *client.UpdateTaskRequest, recorder metrics.MetricsRecorder) error {
	var data string
	err := s.db.QueryRowContext(ctx, getTask, req.UUID).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("cannot find task with uuid: %s", req.UUID)
		}
		return err
	}

	var task tasks.Task
	if err = json.Unmarshal([]byte(data), &task); err != nil {
		return err
	}

	if task.Status == tasks.Available {
		task.WorkedBy = req.WorkedBy
		task.StartedAt = time.Now()
	} else if task.WorkedBy != req.WorkedBy {
		return errors.New("task already acquired")
	}

	log.Infow("state update", "uuid", task.UUID, "status", req.Status, "worked_by", req.WorkedBy)

	task.Status = req.Status

	err = s.updateTask(ctx, &task)
	if err != nil {
		return err
	}
	return recorder.ObserveTask(&task)
}

func (s *state) updateTask(ctx context.Context, task *tasks.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, updateTask, task.UUID, string(data), time.Now())
	return err
}

func (s *state) saveTask(ctx context.Context, task *tasks.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, insertTask, task.UUID, string(data), time.Now())

	return err
}

func (s *state) createInitialTasks(ctx context.Context) error {
	err := s.saveTask(ctx, &tasks.Task{
		UUID:   uuid.New().String()[:8],
		Status: tasks.Available,
		RetrievalTask: &tasks.RetrievalTask{
			Miner:      "t01000",
			PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
			CARExport:  false,
		},
	})
	if err != nil {
		return err
	}

	err = s.saveTask(ctx, &tasks.Task{
		UUID:   uuid.New().String()[:8],
		Status: tasks.Available,
		RetrievalTask: &tasks.RetrievalTask{
			Miner:      "t01000",
			PayloadCID: "bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg",
			CARExport:  false,
		},
	})
	if err != nil {
		return err
	}

	err = s.saveTask(ctx, &tasks.Task{
		UUID:   uuid.New().String()[:8],
		Status: tasks.Available,
		RetrievalTask: &tasks.RetrievalTask{
			Miner:      "f0127896",
			PayloadCID: "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
			CARExport:  false,
		},
	})
	if err != nil {
		return err
	}

	return s.saveTask(ctx, &tasks.Task{
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
	})
}
