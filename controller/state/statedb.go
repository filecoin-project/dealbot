package state

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	blockformat "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	dagjson "github.com/ipld/go-ipld-prime/codec/dagjson"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/multiformats/go-multicodec"

	// DB interfaces
	"github.com/filecoin-project/dealbot/controller/state/postgresdb"
	"github.com/filecoin-project/dealbot/controller/state/sqlitedb"
	"github.com/lib/pq"

	// DB migrations
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Maximum time to retry transactions that fail due to temporary error
const maxRetryTime = time.Minute

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

var linkProto = cidlink.LinkBuilder{Prefix: cid.Prefix{
	Version:  1,
	Codec:    uint64(multicodec.DagJson),
	MhType:   uint64(multicodec.Sha2_256),
	MhLength: 32,
}}

func serializeToJSON(ctx context.Context, n ipld.Node) (cid.Cid, []byte, error) {
	var data []byte

	storer := func(ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
		buf := bytes.Buffer{}
		return &buf, func(lnk ipld.Link) error {
			data = buf.Bytes()
			return nil
		}, nil
	}

	link, err := linkProto.Build(ctx, ipld.LinkContext{}, n, storer)
	if err != nil {
		return cid.Undef, nil, err
	}

	return link.(cidlink.Link).Cid, data, nil
}

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
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, dbName, dbInstance)
	if err != nil {
		return err
	}

	err = m.Up()
	if err == migrate.ErrNoChange {
		return nil
	}
	return err
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

func (s *stateDB) Store(ctx context.Context) Store {
	return &sdbstore{ctx, s}
}

type sdbstore struct {
	context.Context
	*stateDB
}

func (s *sdbstore) Get(c cid.Cid) (blockformat.Block, error) {
	var data []byte
	err := s.stateDB.transact(s.Context, func(tx *sql.Tx) error {
		loader := txContextLoader(s.Context, tx)
		blkReader, err := loader(cidlink.Link{Cid: c}, ipld.LinkContext{})
		if err != nil {
			return err
		}
		data, err = ioutil.ReadAll(blkReader)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return blockformat.NewBlockWithCid(data, c)
}

func (s *sdbstore) Head() (cid.Cid, error) {
	var headCid cid.Cid
	err := s.stateDB.transact(s.Context, func(tx *sql.Tx) error {
		var head string
		err := tx.QueryRowContext(s.Context, queryHeadSQL, LATEST_UPDATE, "").Scan(&head)
		if err != nil {
			return err
		}
		headCid, err = cid.Decode(head)
		return err
	})
	if err != nil {
		return cid.Undef, err
	}
	return headCid, nil
}

func (s *sdbstore) Set(c cid.Cid, data []byte) error {
	return s.stateDB.transact(s.Context, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(s.Context, cidArchiveSQL, c.String(), data, time.Now())
		return err
	})
}

func (s *stateDB) db() *sql.DB {
	return s.dbconn.SqlDB()
}

// Get returns a specific task identified by ID
func (s *stateDB) Get(ctx context.Context, taskID string) (tasks.Task, error) {
	var task tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
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
		return nil
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

// Get returns a specific task identified by CID
func (s *stateDB) GetByCID(ctx context.Context, taskCID cid.Cid) (tasks.Task, error) {
	var task tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var serialized string
		err := tx.QueryRowContext(ctx, getTaskByCidSQL, taskCID.KeyString()).Scan(&serialized)
		if err != nil {
			return err
		}

		tp := tasks.Type.Task.NewBuilder()
		if err := dagjson.Decoder(tp, bytes.NewBufferString(serialized)); err != nil {
			return err
		}
		task = tp.Build().(tasks.Task)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

// GetAll queries all tasks from the DB
func (s *stateDB) GetAll(ctx context.Context) ([]tasks.Task, error) {
	var tasklist []tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, getAllTasksSQL)
		if err != nil {
			return err
		}
		defer rows.Close()

		tasklist = nil // reset in case transaction retry
		for rows.Next() {
			var serialized string
			if err = rows.Scan(&serialized); err != nil {
				return err
			}
			tp := tasks.Type.Task.NewBuilder()
			if err = dagjson.Decoder(tp, bytes.NewBufferString(serialized)); err != nil {
				return err
			}
			task := tp.Build().(tasks.Task)
			tasklist = append(tasklist, task)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tasklist, nil

}

// AssignTask finds the oldest available (unassigned) task and
// assigns it to req.WorkedBy. If there are no available tasks, the task returned
// is nil.
//
// If the request specifies no tags, then this selects any task.  If the
// request specifies tags, then this selects any task with a tag matching one
// of the tags in the request, or any untagged task.
func (s *stateDB) AssignTask(ctx context.Context, req tasks.PopTask) (tasks.Task, error) {
	if req.WorkedBy.String() == "" {
		return nil, errors.New("PopTask request must specify WorkedBy")
	}
	if req.Status == *tasks.Available {
		return nil, fmt.Errorf("cannot assign %q status to task", req.Status.String())
	}

	var tags []interface{}
	if req.Tags.Exists() {
		reqTags := req.Tags.Must()
		tags = make([]interface{}, 0, reqTags.Length())
		iter := reqTags.Iterator()
		for !iter.Done() {
			_, v := iter.Next()
			t := strings.TrimSpace(v.String())
			if t == "" {
				continue
			}
			tags = append(tags, t)
		}
	}

	var assigned tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var cnt int
		err := tx.QueryRowContext(ctx, drainedQuerySQL, req.WorkedBy.String()).Scan(&cnt)
		if err != nil {
			return err
		}

		if cnt > 0 {
			// worker is being drained
			return nil
		}

		var taskID, serialized string
		if len(tags) == 0 {
			err = tx.QueryRowContext(ctx, oldestAvailableTaskSQL).Scan(&taskID, &serialized)
		} else {
			if s.dbconn.Name() == "postgres" {
				err = tx.QueryRowContext(ctx, oldestAvailableTaskWithTagsSQL, pq.Array(tags)).Scan(&taskID, &serialized)
			} else {
				// This SQL formation is needed for sqlite.  Is there a better way?
				sql := fmt.Sprintf(oldestAvailableTaskWithTagsSQLsqlite, "?"+strings.Repeat(",?", len(tags)-1))
				err = tx.QueryRowContext(ctx, sql, tags...).Scan(&taskID, &serialized)
			}
		}
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

		assigned = task.Assign(req.WorkedBy.String(), &req.Status)

		lnk, data, err := serializeToJSON(ctx, assigned.Representation())
		if err != nil {
			return err
		}

		// Assign task to worker
		_, err = tx.ExecContext(ctx, assignTaskSQL, taskID, data, req.WorkedBy.String(), lnk.String())
		if err != nil {
			return fmt.Errorf("could not assign task: %w", err)
		}

		// Set new status for task
		_, err = tx.ExecContext(ctx, setTaskStatusSQL, taskID, req.Status.Int(), time.Now())
		if err != nil {
			return fmt.Errorf("could not update status task: %w", err)
		}

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

func (s *stateDB) Update(ctx context.Context, taskID string, req tasks.UpdateTask) (tasks.Task, error) {
	var task tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
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

		updatedTask, err := task.UpdateTask(req)
		if err != nil {
			return err
		}

		lnk, data, err := serializeToJSON(ctx, updatedTask.Representation())
		if err != nil {
			return err
		}

		// save the update back to DB
		_, err = tx.ExecContext(ctx, updateTaskDataSQL, taskID, data, lnk.String())
		if err != nil {
			return err
		}

		// publish a task event update as neccesary
		if (req.Stage.Exists() && req.Stage.Must().String() != task.Stage.String()) || (req.Status.Int() != task.Status.Int()) {
			runCount := updatedTask.RunCount.Int()
			if runCount < 1 {
				return errors.New("runCount must be at least 1")
			}

			_, err = tx.ExecContext(ctx, upsertTaskStatusSQL, taskID, updatedTask.Status.Int(), updatedTask.Stage.String(), runCount, time.Now())
			if err != nil {
				return err
			}
		}

		// finish if neccesary
		if updatedTask.Status == *tasks.Successful || updatedTask.Status == *tasks.Failed {
			finalized, err := updatedTask.Finalize(ctx, txContextStorer(ctx, tx))
			if err != nil {
				return err
			}
			flink, err := linkProto.Build(ctx, ipld.LinkContext{}, finalized.Representation(), txContextStorer(ctx, tx))
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, addHeadSQL,
				flink.(cidlink.Link).Cid.String(),
				time.Now(),
				req.WorkedBy.String(),
				UNATTACHED_RECORD); err != nil {
				return err
			}
		}

		task = updatedTask
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

func txContextStorer(ctx context.Context, tx *sql.Tx) ipld.Storer {
	return func(ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
		buf := bytes.Buffer{}
		return &buf, func(lnk ipld.Link) error {
			_, err := tx.ExecContext(ctx, cidArchiveSQL, lnk.(cidlink.Link).Cid.String(), buf.Bytes(), time.Now())
			return err
		}, nil
	}
}
func txContextLoader(ctx context.Context, tx *sql.Tx) ipld.Loader {
	return func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		lc := lnk.(cidlink.Link).Cid.String()
		buf := []byte{}
		if err := tx.QueryRowContext(ctx, cidGetArchiveSQL, lc).Scan(&buf); err != nil {
			return nil, err
		}
		return bytes.NewBuffer(buf), nil
	}
}

func (s *stateDB) NewStorageTask(ctx context.Context, storageTask tasks.StorageTask) (tasks.Task, error) {
	task := tasks.Type.Task.New(nil, storageTask)
	var tag string
	if storageTask.Tag.Exists() {
		tag = storageTask.Tag.Must().String()
	}
	// save the update back to DB
	if err := s.saveTask(ctx, task, tag); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *stateDB) NewRetrievalTask(ctx context.Context, retrievalTask tasks.RetrievalTask) (tasks.Task, error) {
	task := tasks.Type.Task.New(retrievalTask, nil)
	var tag string
	if retrievalTask.Tag.Exists() {
		tag = retrievalTask.Tag.Must().String()
	}

	// save the update back to DB
	if err := s.saveTask(ctx, task, tag); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *stateDB) TaskHistory(ctx context.Context, taskID string) ([]tasks.TaskEvent, error) {
	var history []tasks.TaskEvent
	err := s.transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, taskHistorySQL, taskID)
		if err != nil {
			return err
		}
		defer rows.Close()

		history = nil // reset in case transaction retry
		for rows.Next() {
			var status, run int
			var ts time.Time
			var stage string
			if err = rows.Scan(&status, &stage, &run, &ts); err != nil {
				return err
			}
			history = append(history, tasks.TaskEvent{
				Status: tasks.Type.Status.Of(status),
				Stage:  stage,
				Run:    run,
				At:     ts,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return history, nil
}

func (s *stateDB) transact(ctx context.Context, f func(*sql.Tx) error) error {
	// Check connection and reconnect if down
	err := s.dbconn.Connect()
	if err != nil {
		return err
	}

	// SQLite is not safe to use with multiple goroutines concurrently, so mutex
	// lock around transaction is needed.
	var needLock bool
	if s.dbconn.Name() == "sqlite" {
		needLock = true
	}

	var start time.Time
	for {
		if needLock {
			s.txlock.Lock()
		}
		err = withTransaction(ctx, s.db(), f)
		if needLock {
			s.txlock.Unlock()
		}
		if err != nil {
			if s.dbconn.RetryableError(err) && time.Since(start) < maxRetryTime {
				if start.IsZero() {
					start = time.Now()
				}
				log.Warnw("retrying transaction after error", "err", err)
				continue
			}
			return err
		}
		return nil
	}
}

func withTransaction(ctx context.Context, db *sql.DB, f func(*sql.Tx) error) (err error) {
	var tx *sql.Tx
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
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
func (s *stateDB) saveTask(ctx context.Context, task tasks.Task, tag string) error {
	lnk, data, err := serializeToJSON(ctx, task.Representation())
	if err != nil {
		return err
	}

	var tagCol sql.NullString
	if tag != "" {
		tagCol = sql.NullString{
			String: tag,
			Valid:  true,
		}
	}

	return s.transact(ctx, func(tx *sql.Tx) error {
		now := time.Now()
		if _, err := tx.ExecContext(ctx, createTaskSQL, task.UUID.String(), data, now, lnk.String(), tagCol); err != nil {
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

// GetHead gets the latest record update from the controller.
func (s *stateDB) GetHead(ctx context.Context) (tasks.RecordUpdate, error) {
	var recordUpdate tasks.RecordUpdate
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var c string
		err := tx.QueryRowContext(ctx, queryHeadSQL, LATEST_UPDATE, "").Scan(&c)
		if err != nil {
			return err
		}
		cidLink, err := cid.Decode(c)
		if err != nil {
			return err
		}

		na := tasks.Type.RecordUpdate.NewBuilder()
		loader := txContextLoader(ctx, tx)
		if err = (cidlink.Link{Cid: cidLink}).Load(ctx, ipld.LinkContext{}, na, loader); err != nil {
			return err
		}
		recordUpdate = na.Build().(tasks.RecordUpdate)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return recordUpdate, nil
}

// find all unattached records from worker, collect them into a new record update, and make it the new head.
func (s *stateDB) PublishRecordsFrom(ctx context.Context, worker string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		var head string
		var headCid cid.Cid
		err := tx.QueryRowContext(ctx, queryHeadSQL, LATEST_UPDATE, "").Scan(&head)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}
			head = ""
			headCid = cid.Undef
		} else {
			headCid, err = cid.Decode(head)
			if err != nil {
				return err
			}
		}
		headCidSig, err := s.PrivKey.Sign([]byte(head))
		if err != nil {
			return err
		}

		itms, err := tx.QueryContext(ctx, queryHeadSQL, UNATTACHED_RECORD, worker)
		if err != nil {
			return err
		}

		rcrds := []tasks.AuthenticatedRecord{}

		updateHeadStmt, err := tx.PrepareContext(ctx, updateHeadSQL)
		if err != nil {
			return err
		}
		defer updateHeadStmt.Close()

		for itms.Next() {
			var lnk string
			if err := itms.Scan(&lnk); err != nil {
				return err
			}

			_, err = updateHeadStmt.ExecContext(ctx, ATTACHED_RECORD, lnk)
			if err != nil {
				return err
			}
			c, err := cid.Decode(lnk)
			if err != nil {
				return err
			}

			// check if we should include it.
			rcrdRdr, err := txContextLoader(ctx, tx)(cidlink.Link{c}, ipld.LinkContext{})
			if err != nil {
				return err
			}
			tskBuilder := tasks.Type.FinishedTask__Repr.NewBuilder()
			if err := dagjson.Decoder(tskBuilder, rcrdRdr); err != nil {
				return err
			}
			tsk := tskBuilder.Build().(tasks.FinishedTask)
			if tsk.FieldErrorMessage().Exists() {
				em := tsk.FieldErrorMessage().Must().String()
				if strings.Contains(em, "there is an active retrieval deal with") ||
					strings.Contains(em, "blockstore: block not found") ||
					strings.Contains(em, "missing permission to invoke") ||
					strings.Contains(em, "/efs/dealbot") ||
					strings.Contains(em, "handler: websocket connection closed") {
					continue
				}
			}

			sig, err := s.PrivKey.Sign([]byte(lnk))
			if err != nil {
				return err
			}

			rcrd := tasks.Type.AuthenticatedRecord.Of(c, sig)
			rcrds = append(rcrds, rcrd)
		}
		rcrdlst := tasks.Type.List_AuthenticatedRecord.Of(rcrds)

		update := tasks.Type.RecordUpdate.Of(rcrdlst, headCid, headCidSig)
		updateCid, err := linkProto.Build(ctx, ipld.LinkContext{}, update.Representation(), txContextStorer(ctx, tx))
		if err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, addHeadSQL, updateCid.(cidlink.Link).Cid.String(), time.Now(), "", LATEST_UPDATE); err != nil {
			return err
		}
		if head != "" {
			if _, err = tx.ExecContext(ctx, updateHeadSQL, PREVIOUS_UPDATE, head); err != nil {
				return err
			}
		}
		return nil
	})
}

// drainWorker adds a worker to the list of workers to not give work to.
func (s *stateDB) DrainWorker(ctx context.Context, worker string) error {
	if _, err := s.db().ExecContext(ctx, drainedAddSQL, worker); err != nil {
		return err
	}
	return nil
}

// ResetWorkerTasks finds all in progress tasks for a worker and resets them to as if they had never been run
func (s *stateDB) ResetWorkerTasks(ctx context.Context, worker string) error {
	var resetTasks []tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
		inProgressWorkerTasks, err := tx.QueryContext(ctx, workerTasksByStatusSQL, worker, tasks.InProgress.Int())
		if err != nil {
			return err
		}

		for inProgressWorkerTasks.Next() {
			var uuid, serialized string
			err := inProgressWorkerTasks.Scan(&uuid, &serialized)
			if err != nil {
				return err
			}

			tp := tasks.Type.Task.NewBuilder()
			if err = dagjson.Decoder(tp, bytes.NewBufferString(serialized)); err != nil {
				return err
			}
			task := tp.Build().(tasks.Task)

			updatedTask := task.Reset()

			lnk, data, err := serializeToJSON(ctx, updatedTask.Representation())
			if err != nil {
				return err
			}

			// save the update back to DB
			_, err = tx.ExecContext(ctx, unassignTaskSQL, uuid, data, lnk.String())
			if err != nil {
				return err
			}

			// reset the task in the task status ledger
			_, err = tx.ExecContext(ctx, upsertTaskStatusSQL, uuid, updatedTask.Status.Int(), updatedTask.Stage.String(), 0, time.Now())
			if err != nil {
				return err
			}
			resetTasks = append(resetTasks, updatedTask)
		}

		return nil
	})

	if err != nil {
		return err
	}

	if s.recorder != nil && len(resetTasks) > 0 {
		for _, resetTask := range resetTasks {
			if err = s.recorder.ObserveTask(resetTask); err != nil {
				return err
			}
		}
	}
	return nil
}
