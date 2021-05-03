package state

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	logging "github.com/ipfs/go-log/v2"
	dagjson "github.com/ipld/go-ipld-prime/codec/dagjson"
	crypto "github.com/libp2p/go-libp2p-crypto"

	"github.com/filecoin-project/dealbot/controller/state/postgresdb"
	"github.com/filecoin-project/dealbot/controller/state/sqlitedb"
)

var log = logging.Logger("controller-state")

type errorString string

func (e errorString) Error() string {
	return string(e)
}

const ErrNotAssigned = errorString("tasks must be acquired through pop task")
const ErrWrongWorker = errorString("task already acquired by other worker")

// stateDB is a persisted implementation of the State interface
type stateDB struct {
	dbconn DBConnector
	crypto.PrivKey
	recorder metrics.MetricsRecorder
	txlock   sync.Mutex
}

// NewStateDB creates a state instance with a given driver and identity
func NewStateDB(ctx context.Context, driver, conn string, identity crypto.PrivKey, recorder metrics.MetricsRecorder) (State, error) {
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
		return nil, fmt.Errorf("could not create tasks table: %w", err)
	}

	// Create status ledger tablespace if it does not exist
	_, err = db.ExecContext(ctx, createStatusLedgerSQL)
	if err != nil {
		return nil, fmt.Errorf("could not create task_status_ledger table: %w", err)
	}

	st := &stateDB{
		dbconn:   dbConn,
		PrivKey:  identity,
		recorder: recorder,
	}

	return st, nil
}

func (s *stateDB) db() *sql.DB {
	return s.dbconn.SqlDB()
}

// Get returns a specific task identified by ID
func (s *stateDB) Get(ctx context.Context, taskID string) (tasks.Task, error) {
	var serialized string
	err := s.db().QueryRowContext(ctx, getTaskSQL, taskID).Scan(&serialized)
	if err != nil {
		return nil, err
	}

	tp := tasks.Type.Task.NewBuilder()
	if err := dagjson.Decoder(tp, bytes.NewBufferString(serialized)); err != nil {
		return nil, err
	}
	return tp.Build().(tasks.Task), nil
}

// GetAll queries all tasks from the DB
func (s *stateDB) GetAll(ctx context.Context) ([]tasks.Task, error) {
	rows, err := s.db().QueryContext(ctx, getAllTasksSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasklist []tasks.Task
	for rows.Next() {
		var serialized string
		if err = rows.Scan(&serialized); err != nil {
			return nil, err
		}
		tp := tasks.Type.Task.NewBuilder()
		if err = dagjson.Decoder(tp, bytes.NewBufferString(serialized)); err != nil {
			return nil, err
		}
		task := tp.Build().(tasks.Task)
		tasklist = append(tasklist, task)
	}
	return tasklist, nil

}

// AssignTask finds the oldest available (unassigned) task and
// assigns it to req.WorkedBy. If there are no available tasks, the task returned
// is nil.
//
// TODO: There should be a limit to the age of the task to assign.
func (s *stateDB) AssignTask(ctx context.Context, req client.PopTaskRequest) (tasks.Task, error) {
	var assigned tasks.Task
	err := s.transact(ctx, 13, func(tx *sql.Tx) error {
		var taskID, serialized string
		err := tx.QueryRowContext(ctx, oldestAvailableTaskSQL).Scan(&taskID, &serialized)
		if err != nil {
			if err == sql.ErrNoRows {
				// There are no available tasks
				return nil
			}
			return err
		}

		tp := tasks.Type.Task.NewBuilder()
		if err = dagjson.Decoder(tp, bytes.NewBufferString(serialized)); err != nil {
			return err
		}
		task := tp.Build().(tasks.Task)

		task.Assign(req.WorkedBy, req.Status)

		data := bytes.NewBuffer([]byte{})
		if err := dagjson.Encoder(task, data); err != nil {
			return err
		}

		// Assign task to worker
		_, err = tx.ExecContext(ctx, assignTaskSQL, taskID, data, req.WorkedBy)
		if err != nil {
			return err
		}

		// Set new status for task
		_, err = tx.ExecContext(ctx, setTaskStatusSQL, taskID, req.Status, time.Now())
		if err != nil {
			return err
		}

		assigned = task
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.recorder != nil && assigned != nil {
		if err = s.recorder.ObserveTask(assigned); err != nil {
			return nil, err
		}
	}
	return assigned, nil
}

func mustString(s string, _ error) string {
	return s
}

func (s *stateDB) Update(ctx context.Context, taskID string, req client.UpdateTaskRequest) (tasks.Task, error) {
	var task tasks.Task
	err := s.transact(ctx, 3, func(tx *sql.Tx) error {
		var serialized string
		err := tx.QueryRowContext(ctx, getTaskSQL, taskID).Scan(&serialized)
		if err != nil {
			return err
		}

		tp := tasks.Type.Task.NewBuilder()
		if err = dagjson.Decoder(tp, bytes.NewBufferString(serialized)); err != nil {
			return err
		}
		task = tp.Build().(tasks.Task)

		if !task.WorkedBy.Exists() {
			return ErrNotAssigned
		}
		twb := mustString(task.WorkedBy.Must().AsString())
		if twb == "" {
			return ErrNotAssigned
		} else if req.WorkedBy != twb {
			return ErrWrongWorker
		}

		if err := task.Update(req.Status, req.Stage, req.CurrentStageDetails); err != nil {
			return err
		}

		data := bytes.NewBuffer([]byte{})
		if err := dagjson.Encoder(task, data); err != nil {
			return err
		}

		// save the update back to DB
		_, err = tx.ExecContext(ctx, updateTaskDataSQL, taskID, data)
		if err != nil {
			return err
		}

		// publish a task event update as neccesary
		_, err = tx.ExecContext(ctx, upsertTaskStatusSQL, taskID, task.Status, task.Stage, time.Now())
		return nil
	})

	if err != nil {
		return nil, err
	}

	if s.recorder != nil {
		if err = s.recorder.ObserveTask(task); err != nil {
			return nil, err
		}
	}

	return task, nil
}

func (s *stateDB) NewStorageTask(ctx context.Context, storageTask tasks.StorageTask) (tasks.Task, error) {
	task := tasks.Type.Task.New(nil, storageTask)

	// save the update back to DB
	if err := s.saveTask(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *stateDB) NewRetrievalTask(ctx context.Context, retrievalTask tasks.RetrievalTask) (tasks.Task, error) {
	task := tasks.Type.Task.New(retrievalTask, nil)

	// save the update back to DB
	if err := s.saveTask(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *stateDB) TaskHistory(ctx context.Context, taskID string) ([]tasks.TaskEvent, error) {
	rows, err := s.db().QueryContext(ctx, taskHistorySQL, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []tasks.TaskEvent
	for rows.Next() {
		var status int
		var ts time.Time
		var stage string
		if err = rows.Scan(&status, &stage, &ts); err != nil {
			return nil, err
		}
		history = append(history, tasks.TaskEvent{tasks.Type.Status.Of(status), stage, ts})
	}
	return history, nil
}

func (s *stateDB) createInitialTasks(ctx context.Context) error {
	rt := tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36", false)
	task := tasks.Type.Task.New(rt, nil)
	err := s.saveTask(ctx, task)
	if err != nil {
		return err
	}

	rt = tasks.Type.RetrievalTask.Of("t01000", "bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg", false)
	task = tasks.Type.Task.New(rt, nil)
	err = s.saveTask(ctx, task)
	if err != nil {
		return err
	}

	rt = tasks.Type.RetrievalTask.Of("f0127896", "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm", false)
	task = tasks.Type.Task.New(rt, nil)
	err = s.saveTask(ctx, task)
	if err != nil {
		return err
	}

	st := tasks.Type.StorageTask.Of(
		"t01000",
		100000000000000000, // 0.10 FIL
		1024,               // 1kb
		0,
		true,
		false)
	task = tasks.Type.Task.New(nil, st)
	return s.saveTask(ctx, task)
}

func (s *stateDB) transact(ctx context.Context, retries int, f func(*sql.Tx) error) (err error) {
	// Check connection and reconnect if down
	err = s.dbconn.Connect()
	if err != nil {
		return
	}

	s.txlock.Lock()
	defer s.txlock.Unlock()

	var tx *sql.Tx
	if tx, err = s.db().BeginTx(ctx, nil); err != nil {
		return
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = f(tx)
	return
}

// createTask inserts a new task and new task status into the database
func (s *stateDB) saveTask(ctx context.Context, task tasks.Task) error {
	data := bytes.NewBuffer([]byte{})
	if err := dagjson.Encoder(task, data); err != nil {
		return err
	}

	return s.transact(ctx, 0, func(tx *sql.Tx) error {
		now := time.Now()
		if _, err := tx.ExecContext(ctx, createTaskSQL, task.UUID, data, now); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, setTaskStatusSQL, task.UUID, tasks.Available, now); err != nil {
			return err
		}
		return nil
	})
}

// countTasks retrieves the total number of tasks
func (s *stateDB) countTasks(ctx context.Context) (int, error) {
	var count int
	if err := s.db().QueryRowContext(ctx, countAllTasksSQL).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
