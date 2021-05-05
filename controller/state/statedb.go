package state

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	logging "github.com/ipfs/go-log/v2"
	dagjson "github.com/ipld/go-ipld-prime/codec/dagjson"
	crypto "github.com/libp2p/go-libp2p-crypto"

	// DB interfaces
	"github.com/filecoin-project/dealbot/controller/state/postgresdb"
	"github.com/filecoin-project/dealbot/controller/state/sqlitedb"

	// DB migrations
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
	//"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Embed all the *.sql files in migrations.
//go:embed migrations/*.sql
var migrations embed.FS

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

func migratePostgres(db *sql.DB) error {
	dbInstance, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}
	return migrateDatabase("postgres", dbInstance)
}

func migrateSqlite(db *sql.DB) error {
	dbInstance, err := sqlite.WithInstance(db, &sqlite.Config{NoTxWrap: true})
	if err != nil {
		return err
	}
	return migrateDatabase("sqlite", dbInstance)
}

func migrateDatabase(dbName string, dbInstance database.Driver) error {
	// TODO: Replace httpfs with iofs when it becomes available (June 2021?)
	source, err := httpfs.New(http.FS(migrations), "migrations")
	//source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, dbName, dbInstance)
	if err != nil {
		return err
	}
	//return m.Steps(2) // Migrate 2 versions up at mose
	return m.Up()
}

// NewStateDB creates a state instance with a given driver and identity
func NewStateDB(ctx context.Context, driver, conn string, identity crypto.PrivKey, recorder metrics.MetricsRecorder) (State, error) {
	var dbConn DBConnector
	var migrateFunc func(*sql.DB) error

	switch driver {
	case "postgres":
		if conn == "" {
			conn = postgresdb.PostgresConfig{}.String()
		}
		dbConn = postgresdb.New(conn)
		migrateFunc = migratePostgres
	case "sqlite":
		dbConn = sqlitedb.New(conn)
		migrateFunc = migrateSqlite
	default:
		return nil, fmt.Errorf("database driver %q is not supported", driver)
	}

	// Open database connection
	err := dbConn.Connect()
	if err != nil {
		return nil, err
	}
	db := dbConn.SqlDB()

	// Apply DB schema migrations
	if err = migrateFunc(db); err != nil {
		return nil, fmt.Errorf("%s database migration failed: %w", driver, err)
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
func (s *stateDB) AssignTask(ctx context.Context, req tasks.PopTask) (tasks.Task, error) {
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
			return fmt.Errorf("could not decode task (%s): %w", serialized, err)
		}
		task := tp.Build().(tasks.Task)

		task.Assign(req.WorkedBy.String(), &req.Status)

		data := bytes.NewBuffer([]byte{})
		if err := dagjson.Encoder(task.Representation(), data); err != nil {
			return err
		}

		// Assign task to worker
		_, err = tx.ExecContext(ctx, assignTaskSQL, taskID, data.Bytes(), req.WorkedBy.String())
		if err != nil {
			return fmt.Errorf("could not assign task: %w", err)
		}

		// Set new status for task
		_, err = tx.ExecContext(ctx, setTaskStatusSQL, taskID, req.Status.Int(), time.Now())
		if err != nil {
			return fmt.Errorf("could not update status task: %w", err)
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

func (s *stateDB) Update(ctx context.Context, taskID string, req tasks.UpdateTask) (tasks.Task, error) {
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
		twb := task.WorkedBy.Must().String()
		if twb == "" {
			return ErrNotAssigned
		} else if req.WorkedBy.String() != twb {
			return ErrWrongWorker
		}

		if err := task.UpdateTask(req); err != nil {
			return err
		}

		data := bytes.NewBuffer([]byte{})
		if err := dagjson.Encoder(task.Representation(), data); err != nil {
			return err
		}

		// save the update back to DB
		_, err = tx.ExecContext(ctx, updateTaskDataSQL, taskID, data.Bytes())
		if err != nil {
			return err
		}

		// publish a task event update as neccesary
		now := time.Now()
		if req.CurrentStageDetails.Exists() && req.CurrentStageDetails.Must().UpdatedAt.Exists() {
			now = req.CurrentStageDetails.Must().UpdatedAt.Must().Time()
		}
		_, err = tx.ExecContext(ctx, upsertTaskStatusSQL, taskID, task.Status.Int(), task.Stage.String(), now)
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
	if err := dagjson.Encoder(task.Representation(), data); err != nil {
		return err
	}

	return s.transact(ctx, 0, func(tx *sql.Tx) error {
		now := time.Now()
		if _, err := tx.ExecContext(ctx, createTaskSQL, task.UUID.String(), data.Bytes(), now); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, setTaskStatusSQL, task.UUID.String(), tasks.Available.Int(), now); err != nil {
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
