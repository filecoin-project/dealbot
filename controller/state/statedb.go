package state

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
	logging "github.com/ipfs/go-log/v2"
	crypto "github.com/libp2p/go-libp2p-crypto"

	"github.com/filecoin-project/dealbot/controller/state/postgresdb"
	"github.com/filecoin-project/dealbot/controller/state/sqlitedb"
)

var log = logging.Logger("controller-state")

// stateDB is a persisted implementation of the State interface
type stateDB struct {
	dbconn DBConnector
	crypto.PrivKey
}

// NewStateDB creates a state instance with a given driver and identity
func NewStateDB(ctx context.Context, driver, conn string, identity crypto.PrivKey) (State, error) {
	var dbConn DBConnector
	switch driver {
	case "postgres":
		if conn == "" {
			conn = postgresdb.PostgresConfig{}.String()
		}
		dbConn = postgresdb.New(conn)
	case "sqlite":
		dbConn = sqlitedb.New(conn)
	default:
		return nil, fmt.Errorf("database driver %q is not supported", driver)
	}

	// Open database connection
	err := dbConn.Connect()
	if err != nil {
		return nil, err
	}
	db := dbConn.SqlDB()

	// Create state tablespace if it does not exist
	_, err = db.ExecContext(ctx, createTasksTableSQL)
	if err != nil {
		return nil, err
	}

	st := &stateDB{
		dbconn:  dbConn,
		PrivKey: identity,
	}

	return st, nil
}

func (s *stateDB) db() *sql.DB {
	return s.dbconn.SqlDB()
}

// Get queries the DB for the task identified by UUID
func (s *stateDB) Get(ctx context.Context, uuid string) (*tasks.AuthenticatedTask, error) {
	var serialized string
	err := s.db().QueryRowContext(ctx, getTaskSQL, uuid).Scan(&serialized)
	if err != nil {
		return nil, err
	}

	var task tasks.AuthenticatedTask
	if err := json.Unmarshal([]byte(serialized), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// GetAll queries all tasks from the DB
func (s *stateDB) GetAll(ctx context.Context) ([]*tasks.AuthenticatedTask, error) {
	rows, err := s.db().QueryContext(ctx, getLatestTasksSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasklist []*tasks.AuthenticatedTask
	for rows.Next() {
		var serialized string
		if err = rows.Scan(&serialized); err != nil {
			return nil, err
		}
		var task tasks.AuthenticatedTask
		if err = json.Unmarshal([]byte(serialized), &task); err != nil {
			return nil, err
		}
		tasklist = append(tasklist, &task)
	}
	return tasklist, nil

}

func (s *stateDB) Update(ctx context.Context, uuid string, req *client.UpdateTaskRequest, recorder metrics.MetricsRecorder) (*tasks.AuthenticatedTask, error) {
	// Check connection and reconnect if down
	err := s.dbconn.Connect()
	if err != nil {
		return nil, err
	}

	latest, err := s.Get(ctx, uuid)
	if err != nil {
		return nil, err
	}

	if latest.Status == tasks.Available {
		latest.WorkedBy = req.WorkedBy
		latest.StartedAt = time.Now()
	} else if latest.WorkedBy != req.WorkedBy {
		return nil, errors.New("task already acquired")
	}
	log.Infow("state update", "uuid", latest.UUID, "status", req.Status, "worked_by", req.WorkedBy)

	latest.Status = req.Status
	latest.Signature, err = s.PrivKey.Sign(latest.Bytes())
	if err != nil {
		return nil, err
	}

	// save the update back to DB
	if err := s.saveTask(ctx, latest); err != nil {
		return nil, err
	}

	if err := recorder.ObserveTask(latest); err != nil {
		return nil, err
	}

	return latest, nil

}

// save inserts a new task into the database
func (s *stateDB) saveTask(ctx context.Context, task *tasks.AuthenticatedTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// Not every db driver may support getting rows affected, so ignore results.
	_, err = s.db().ExecContext(ctx, insertTaskSQL, task.UUID, data, time.Now())

	return err
}

func (s *stateDB) countTasks(ctx context.Context) (int, error) {
	var count int
	if err := s.db().QueryRowContext(ctx, countTasksSQL).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *stateDB) NewStorageTask(ctx context.Context, storageTask *tasks.StorageTask) (*tasks.AuthenticatedTask, error) {
	task := &tasks.AuthenticatedTask{
		Task: tasks.Task{
			UUID:        uuid.New().String()[:8],
			Status:      tasks.Available,
			StorageTask: storageTask,
		},
		Signature: []byte{},
	}
	var err error
	task.Signature, err = s.PrivKey.Sign(task.Bytes())
	if err != nil {
		return nil, err
	}

	// save the update back to DB
	if err = s.saveTask(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *stateDB) NewRetrievalTask(ctx context.Context, retrievalTask *tasks.RetrievalTask) (*tasks.AuthenticatedTask, error) {
	task := &tasks.AuthenticatedTask{
		Task: tasks.Task{
			UUID:          uuid.New().String()[:8],
			Status:        tasks.Available,
			RetrievalTask: retrievalTask,
		},
		Signature: []byte{},
	}
	var err error
	task.Signature, err = s.PrivKey.Sign(task.Bytes())
	if err != nil {
		return nil, err
	}

	// save the update back to DB
	if err = s.saveTask(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}
