package state

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	dagjson "github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/multiformats/go-multicodec"
	tokenjson "github.com/polydawn/refmt/json"
	"github.com/robfig/cron/v3"
	dumpjson "github.com/willscott/ipld-dumpjson"

	// DB interfaces

	"github.com/lib/pq"
	// DB migrations
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
const ErrNoDeleteInProgressTasks = errorString("can only delete tasks that are not-started or tasks that are scheduled")
const ErrTaskNotFound = errorString("task does not exist")

var linkProto = cidlink.LinkPrototype{Prefix: cid.Prefix{
	Version:  1,
	Codec:    uint64(multicodec.DagJson),
	MhType:   uint64(multicodec.Sha2_256),
	MhLength: 32,
}}

func serializeToJSON(ctx context.Context, n ipld.Node) (cid.Cid, []byte, error) {
	var data []byte

	ls := cidlink.DefaultLinkSystem()
	ms := memstore.Store{}
	ls.SetWriteStorage(&ms)
	ls.SetReadStorage(&ms)

	link, err := ls.Store(ipld.LinkContext{}, linkProto, n)
	if err != nil {
		return cid.Undef, nil, err
	}

	cl := link.(cidlink.Link).Cid

	data, err = ms.Get(ctx, cl.KeyString())
	if err != nil {
		return cid.Undef, nil, err
	}

	return cl, data, nil
}

var _ State = (*stateDB)(nil)

// stateDB is a persisted implementation of the State interface
type stateDB struct {
	dbconn    DBConnector
	cronSched *cron.Cron
	crypto.PrivKey
	recorder  metrics.MetricsRecorder
	outlog    io.WriteCloser
	runNotice chan string
}

// NewStateDB creates a state instance with a given driver and identity
func NewStateDB(ctx context.Context, dbConn DBConnector, migrator Migrator, logfile string, identity crypto.PrivKey, recorder metrics.MetricsRecorder) (State, error) {
	return newStateDBWithNotify(ctx, dbConn, migrator, logfile, identity, recorder, nil)
}

// newStateDBWithNotify is NewStateDB with additional parameters for testing
func newStateDBWithNotify(ctx context.Context, dbConn DBConnector, migrator Migrator, logfile string, identity crypto.PrivKey, recorder metrics.MetricsRecorder, runNotice chan string) (State, error) {

	// Open database connection
	err := dbConn.Connect()
	if err != nil {
		return nil, err
	}
	db := dbConn.SqlDB()

	// Apply DB schema migrations
	if err = migrator(db); err != nil {
		return nil, fmt.Errorf("%s database migration failed: %w", dbConn.Name(), err)
	}

	//cronSched := cron.New(cron.WithSeconds())
	cronSched := cron.New()
	cronSched.Start()

	var outlog io.WriteCloser = &discard{}
	if logfile != "" {
		outlog, err = os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open output log: %w", err)
		}
	}

	st := &stateDB{
		dbconn:    dbConn,
		cronSched: cronSched,
		PrivKey:   identity,
		recorder:  recorder,
		runNotice: runNotice,
		outlog:    outlog,
	}

	count, err := st.recoverScheduledTasks(ctx)
	if err != nil {
		return nil, err
	}
	if count != 0 {
		log.Infow("recovered scheduled tasks", "task_count", count)
	}

	return st, nil
}

type discard struct{}

func (d *discard) Write(p []byte) (int, error) {
	return len(p), nil
}

func (d *discard) Close() error {
	return nil
}

func (s *stateDB) Store(ctx context.Context) Store {
	return &sdbstore{ctx, s}
}

type sdbstore struct {
	context.Context
	*stateDB
}

func (s *sdbstore) Get(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := s.stateDB.transact(s.Context, func(tx *sql.Tx) error {
		ls := txLS(s.Context, tx)
		var err error
		ci, err := cid.Decode(key)
		if err != nil {
			return err
		}
		_, data, err = ls.LoadPlusRaw(ipld.LinkContext{}, cidlink.Link{Cid: ci}, basicnode.Prototype.Any)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return data, err
}

func (s *sdbstore) Has(ctx context.Context, key string) (bool, error) {
	err := s.stateDB.transact(s.Context, func(tx *sql.Tx) error {
		ls := txLS(s.Context, tx)
		var err error
		v, err := cid.Decode(key)
		if err != nil {
			return err
		}
		_, _, err = ls.LoadPlusRaw(ipld.LinkContext{}, cidlink.Link{Cid: v}, basicnode.Prototype.Any)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

func (s *sdbstore) Put(_ context.Context, key string, content []byte) error {
	return s.stateDB.transact(s.Context, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(s.Context, cidArchiveSQL, key, content, time.Now())
		if err != nil {
			return fmt.Errorf("failed to archive %s ('%s'): %w", key, content, err)
		}
		return err
	})
}

func (s *stateDB) db() *sql.DB {
	return s.dbconn.SqlDB()
}

// Get returns a specific task identified by ID
func (s *stateDB) Get(ctx context.Context, taskID string) (*tasks.Task, error) {
	task, _, err := s.getWithTag(ctx, taskID)
	return task, err
}

func (s *stateDB) getWithTag(ctx context.Context, taskID string) (*tasks.Task, string, error) {
	var task *tasks.Task
	var tag string
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var serialized string
		var tagString sql.NullString
		err := tx.QueryRowContext(ctx, getTaskWithTagSQL, taskID).Scan(&serialized, &tagString)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		tp := tasks.TaskPrototype.NewBuilder()
		if err = dagjson.Decode(tp, bytes.NewBufferString(serialized)); err != nil {
			return err
		}
		task, err = tasks.UnwrapTask(tp.Build())
		if err != nil {
			return err
		}

		if tagString.Valid {
			tag = tagString.String
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return task, tag, nil
}

// Get returns a specific task identified by CID
func (s *stateDB) GetByCID(ctx context.Context, taskCID cid.Cid) (*tasks.Task, error) {
	var task *tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var serialized string
		err := tx.QueryRowContext(ctx, getTaskByCidSQL, taskCID.KeyString()).Scan(&serialized)
		if err != nil {
			return err
		}

		tp := tasks.TaskPrototype.NewBuilder()
		if err := dagjson.Decode(tp, bytes.NewBufferString(serialized)); err != nil {
			return err
		}
		task, err = tasks.UnwrapTask(tp.Build())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

// GetAll queries all tasks from the DB
func (s *stateDB) GetAll(ctx context.Context) ([]*tasks.Task, error) {
	var tasklist []*tasks.Task
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
			tp := tasks.TaskPrototype.NewBuilder()
			if err = dagjson.Decode(tp, bytes.NewBufferString(serialized)); err != nil {
				return err
			}
			task, err := tasks.UnwrapTask(tp.Build())
			if err != nil {
				return err
			}
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
func (s *stateDB) AssignTask(ctx context.Context, req *tasks.PopTask) (*tasks.Task, error) {
	if req.WorkedBy == "" {
		return nil, errors.New("PopTask request must specify WorkedBy")
	}
	if req.Status == tasks.Available {
		return nil, fmt.Errorf("cannot assign %q status to task", req.Status.String())
	}

	// Check if this worker is being drained
	draining, err := s.drainingWorker(ctx, req.WorkedBy)
	if err != nil {
		return nil, err
	}
	if draining {
		return nil, nil
	}

	var tags []interface{}
	if req.Tags != nil {
		reqTags := req.Tags
		tags = make([]interface{}, 0, int64(len(reqTags)))
		for _, v := range reqTags {
			t := strings.TrimSpace(v)
			if t == "" {
				continue
			}
			tags = append(tags, t)
		}
	}

	// Keep popping tasks until there a runanable task is found or until there
	// are no more tasks
	var assigned *tasks.Task
	for {
		task, scheduled, err := s.popTask(ctx, req.WorkedBy, req.Status, tags)
		if err != nil {
			return nil, err
		}
		if task == nil {
			// No more tasks to pop
			return nil, nil
		}
		if !scheduled {
			// Found a task that is runable, stop looking for tasks
			assigned = task
			break
		}
		// Got a scheduled task, so schedule it.
		err = s.scheduleTask(task)
		if err != nil {
			log.Errorw("failed to schedule task", "taskID", task.UUID, "err", err)
		}
	}

	if s.recorder != nil {
		if err = s.recorder.ObserveTask(assigned); err != nil {
			return nil, err
		}
	}
	return assigned, nil
}

func (s *stateDB) drainingWorker(ctx context.Context, worker string) (bool, error) {
	var draining bool
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var cnt int
		err := tx.QueryRowContext(ctx, drainedQuerySQL, worker).Scan(&cnt)
		if err != nil {
			return err
		}

		if cnt > 0 {
			// worker is being drained
			draining = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return draining, nil
}

func (s *stateDB) popTask(ctx context.Context, workedBy string, status tasks.Status, tags []interface{}) (*tasks.Task, bool, error) {
	var assigned *tasks.Task
	var scheduled bool

	err := s.transact(ctx, func(tx *sql.Tx) error {
		var taskID, serialized string
		var err error
		if len(tags) == 0 {
			err = tx.QueryRowContext(ctx, oldestAvailableTaskSQL).Scan(&taskID, &serialized)
		} else {
			err = tx.QueryRowContext(ctx, oldestAvailableTaskWithTagsSQL, pq.Array(tags)).Scan(&taskID, &serialized)
		}
		if err != nil {
			if err == sql.ErrNoRows {
				// There are no available tasks
				return nil
			}
			return err
		}

		tp := tasks.TaskPrototype.NewBuilder()
		if err = dagjson.Decode(tp, bytes.NewBufferString(serialized)); err != nil {
			return fmt.Errorf("could not decode task (%s): %w", serialized, err)
		}
		task, err := tasks.UnwrapTask(tp.Build())
		if err != nil {
			return err
		}

		if task.HasSchedule() {
			// This task has a schedule, but is not owned by the scheduler.
			// Normally tasks are scheduled on ingestion, before they are
			// written to the DB.  However, this may have been a previously
			// scheduled task that became that was rescheduled or inserted into
			// the DB outside of the notmal ingestion process.
			log.Warnw("found scheduled task with no owner, scheduling task", taskID)

			// Assign task to scheduler in DB only
			_, err = tx.ExecContext(ctx, updateTaskWorkedBySQL, taskID, schedulerOwner)
			if err != nil {
				return fmt.Errorf("could not assign task: %w", err)
			}
			assigned = task
			scheduled = true
			return nil
		}

		assigned = task.Assign(workedBy, status)

		n, err := assigned.ToNode()
		if err != nil {
			return err
		}

		lnk, data, err := serializeToJSON(ctx, n)
		if err != nil {
			return err
		}

		// Assign task to worker
		_, err = tx.ExecContext(ctx, assignTaskSQL, taskID, data, workedBy, lnk.String())
		if err != nil {
			return fmt.Errorf("could not assign task: %w", err)
		}

		// Set new status for task
		_, err = tx.ExecContext(ctx, setTaskStatusSQL, taskID, int64(status), time.Now())
		if err != nil {
			return fmt.Errorf("could not update status task: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return assigned, scheduled, nil
}

func (s *stateDB) Update(ctx context.Context, taskID string, req *tasks.UpdateTask) (*tasks.Task, error) {
	var task *tasks.Task

	var downstreamRetrieval *tasks.RetrievalTask

	err := s.transact(ctx, func(tx *sql.Tx) error {
		var serialized string
		err := tx.QueryRowContext(ctx, getTaskSQL, taskID).Scan(&serialized)
		if err != nil {
			return err
		}

		tp := tasks.TaskPrototype.NewBuilder()
		if err = dagjson.Decode(tp, bytes.NewBufferString(serialized)); err != nil {
			return err
		}
		task, err = tasks.UnwrapTask(tp.Build())
		if err != nil {
			return err
		}

		if task.WorkedBy == nil {
			return ErrNotAssigned
		}
		twb := *(task.WorkedBy)
		if twb == "" {
			return ErrNotAssigned
		} else if req.WorkedBy != twb {
			return ErrWrongWorker
		}

		updatedTask, err := task.UpdateTask(req)
		if err != nil {
			return err
		}

		utNode, err := updatedTask.ToNode()
		if err != nil {
			return err
		}

		lnk, data, err := serializeToJSON(ctx, utNode)
		if err != nil {
			return err
		}

		// save the update back to DB
		_, err = tx.ExecContext(ctx, updateTaskDataSQL, taskID, data, lnk.String())
		if err != nil {
			return err
		}

		// publish a task event update as neccesary
		if (req.Stage != nil && *(req.Stage) != task.Stage) || (req.Status != task.Status) {
			runCount := updatedTask.RunCount
			if runCount < 1 {
				return errors.New("runCount must be at least 1")
			}

			_, err = tx.ExecContext(ctx, upsertTaskStatusSQL, taskID, updatedTask.Status, updatedTask.Stage, runCount, time.Now())
			if err != nil {
				return err
			}
		}

		// finish if neccesary
		if updatedTask.Status == tasks.Successful || updatedTask.Status == tasks.Failed {
			finalized, err := updatedTask.Finalize(ctx, txLS(ctx, tx), false)
			if err != nil {
				return err
			}
			fnode, err := finalized.ToNode()
			if err != nil {
				return err
			}

			ls := txLS(ctx, tx)
			flink, err := ls.Store(ipld.LinkContext{}, linkProto, fnode)
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, addHeadSQL,
				flink.(cidlink.Link).Cid.String(),
				time.Now(),
				req.WorkedBy,
				UNATTACHED_RECORD); err != nil {
				return err
			}

			if updatedTask.Status == tasks.Successful &&
				finalized.StorageTask != nil &&
				finalized.StorageTask.RetrievalSchedule != nil {
				// we're in a tx context already. we'll enqueue these and put them in non-atomically.
				tag := ""
				if finalized.StorageTask.Tag != nil {
					tag = *(finalized.StorageTask.Tag)
				}
				var maxPrice int64 = 0
				if finalized.StorageTask.RetrievalMaxPriceAttoFIL != nil {
					maxPrice = *(finalized.StorageTask.RetrievalMaxPriceAttoFIL)
				}

				ret := tasks.NewRetrievalTaskWithSchedule(
					finalized.StorageTask.Miner,
					*(finalized.PayloadCID),
					false,
					tag,
					*(finalized.StorageTask.RetrievalSchedule),
					*(finalized.StorageTask.RetrievalScheduleLimit),
					maxPrice,
				)
				downstreamRetrieval = ret
			}
		}
		s.log(ctx, updatedTask, tx)

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

	if downstreamRetrieval != nil {
		_, err := s.NewRetrievalTask(ctx, downstreamRetrieval)
		if err != nil {
			return nil, err
		}
	}

	return task, nil
}

func (s *stateDB) log(ctx context.Context, task *tasks.Task, tx *sql.Tx) {
	finalized, err := task.Finalize(ctx, txLS(ctx, tx), true)
	if err != nil {
		return
	}

	fnode, err := finalized.ToNode()
	if err != nil {
		return
	}

	taskBytes := bytes.Buffer{}
	err = dumpjson.Marshal(fnode, tokenjson.NewEncoder(&taskBytes, tokenjson.EncodeOptions{}), true)
	if err != nil {
		return
	}
	var rawJSON json.RawMessage
	if taskBytes.Bytes() != nil {
		if err := json.Unmarshal(taskBytes.Bytes(), &rawJSON); err != nil {
			return
		}
		log.Infow("Task Finalized", "taskID", task.UUID, "taskOutput", rawJSON)
		if _, err := s.outlog.Write(taskBytes.Bytes()); err != nil {
			log.Warnw("could not write to outlog", "error", err)
		}
		_, _ = s.outlog.Write([]byte("\n"))
	}
}

func txLS(ctx context.Context, tx *sql.Tx) ipld.LinkSystem {
	ls := cidlink.DefaultLinkSystem()
	ls.StorageWriteOpener = func(lc linking.LinkContext) (io.Writer, linking.BlockWriteCommitter, error) {
		buf := bytes.Buffer{}
		return &buf, func(lnk ipld.Link) error {
			_, err := tx.ExecContext(ctx, cidArchiveSQL, lnk.(cidlink.Link).Cid.String(), buf.Bytes(), time.Now())
			return err
		}, nil
	}
	ls.StorageReadOpener = func(_ ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		lc := lnk.(cidlink.Link).Cid.String()
		buf := []byte{}
		if err := tx.QueryRowContext(ctx, cidGetArchiveSQL, lc).Scan(&buf); err != nil {
			return nil, err
		}
		return bytes.NewBuffer(buf), nil
	}
	return ls
}

func (s *stateDB) NewStorageTask(ctx context.Context, storageTask *tasks.StorageTask) (*tasks.Task, error) {
	task := tasks.NewTask(nil, storageTask)
	var tag string
	if storageTask.Tag != nil {
		tag = *(storageTask.Tag)
	}
	// save the update back to DB
	if err := s.saveTask(ctx, task, tag, ""); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *stateDB) NewRetrievalTask(ctx context.Context, retrievalTask *tasks.RetrievalTask) (*tasks.Task, error) {
	task := tasks.NewTask(retrievalTask, nil)
	var tag string
	if retrievalTask.Tag != nil {
		tag = *(retrievalTask.Tag)
	}

	// save the update back to DB
	if err := s.saveTask(ctx, task, tag, ""); err != nil {
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
				Status: tasks.Status(status),
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

	var start time.Time
	for {
		err = withTransaction(ctx, s.db(), f)
		if err != nil {
			if s.dbconn.RetryableError(err) && (start.IsZero() || time.Since(start) < maxRetryTime) {
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
	tx, err = db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
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
func (s *stateDB) saveTask(ctx context.Context, task *tasks.Task, tag, parent string) error {
	tnode, err := task.ToNode()
	if err != nil {
		return err
	}
	lnk, data, err := serializeToJSON(ctx, tnode)
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

	var parentCol sql.NullString
	if parent != "" {
		parentCol = sql.NullString{
			String: parent,
			Valid:  true,
		}
	}

	taskID := task.UUID
	scheduledTask := task.HasSchedule()

	err = s.transact(ctx, func(tx *sql.Tx) error {
		now := time.Now()
		if _, err := tx.ExecContext(ctx, createTaskSQL, taskID, data, now, lnk.String(), tagCol, parentCol); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, setTaskStatusSQL, taskID, int64(tasks.Available), now); err != nil {
			return err
		}

		if scheduledTask {
			// Assign task to scheduler in DB only
			if _, err = tx.ExecContext(ctx, updateTaskWorkedBySQL, taskID, schedulerOwner); err != nil {
				return fmt.Errorf("could not assign task to scheduler: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	if scheduledTask {
		// Got a scheduled task, so schedule it.
		if err = s.scheduleTask(task); err != nil {
			return fmt.Errorf("failed to schedule task %q: %w", taskID, err)
		}
	}
	return nil
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
func (s *stateDB) GetHead(ctx context.Context, walkback int) (*tasks.RecordUpdate, error) {
	var recordUpdate *tasks.RecordUpdate
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var c string
		err := tx.QueryRowContext(ctx, queryHeadSQL, LATEST_UPDATE, "").Scan(&c)
		if err != nil {
			return fmt.Errorf("failed to load latest head: %w", err)
		}
		cidLink, err := cid.Decode(c)
		if err != nil {
			return fmt.Errorf("failed parse head cid: %w", err)
		}

		ls := txLS(ctx, tx)
		ru, raw, err := ls.LoadPlusRaw(ipld.LinkContext{}, cidlink.Link{Cid: cidLink}, tasks.RecordUpdatePrototype)
		if err != nil {
			return fmt.Errorf("failed to parse %s ('%s'): %w", cidLink, raw, err)
		}
		recordUpdate, err = tasks.UnwrapRecordUpdate(ru)
		if err != nil {
			return fmt.Errorf("failed to unwrap RecordUpdate node, %w", err)
		}
		for walkback > 0 {
			if recordUpdate.Previous == nil {
				return fmt.Errorf("no previous update to walk back to")
			}

			ru, err := ls.Load(ipld.LinkContext{}, *(recordUpdate.Previous), tasks.RecordUpdatePrototype)
			if err != nil {
				return err
			}
			recordUpdate, err = tasks.UnwrapRecordUpdate(ru)
			if err != nil {
				return fmt.Errorf("failed to unwrap RecordUpdate node, %w", err)
			}

			walkback--
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return recordUpdate, nil
}

// find all unattached records from worker, collect them into a new record update, and make it the new head.
func (s *stateDB) PublishRecordsFrom(ctx context.Context, worker string) (cid.Cid, error) {
	// Variable to the CID of the stored UpdateRecord.
	var updCid cid.Cid
	// Attempt to publish records with retry in a transaction.
	// This function blocks until either the transaction is completed or returns erroneously.
	err := s.transact(ctx, func(tx *sql.Tx) error {
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

		var lnks []string
		for itms.Next() {
			var lnk string
			if err := itms.Scan(&lnk); err != nil {
				return err
			}
			lnks = append(lnks, lnk)
		}

		var rcrds []*tasks.AuthenticatedRecord

		updateHeadStmt, err := tx.PrepareContext(ctx, updateHeadSQL)
		if err != nil {
			return err
		}
		defer updateHeadStmt.Close()

		for _, lnk := range lnks {
			_, err = updateHeadStmt.ExecContext(ctx, ATTACHED_RECORD, lnk)
			if err != nil {
				return err
			}
			c, err := cid.Decode(lnk)
			if err != nil {
				return err
			}

			// check if we should include it.
			ls := txLS(ctx, tx)
			rcrdRdr, err := ls.Load(ipld.LinkContext{}, cidlink.Link{c}, tasks.FinishedTaskPrototype)
			if err != nil {
				return err
			}
			tsk, err := tasks.UnwrapFinishedTask(rcrdRdr)
			if err != nil {
				return err
			}
			if tsk.ErrorMessage != nil {
				em := *tsk.ErrorMessage
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

			rcrd := tasks.NewAuthenticatedRecord(c, sig)
			rcrds = append(rcrds, rcrd)
		}
		rcrdlst := tasks.NewAuthenticatedRecordList(rcrds)

		update := tasks.NewRecordUpdate(rcrdlst, headCid, headCidSig)
		ls := txLS(ctx, tx)
		unode, err := update.ToNode()
		if err != nil {
			return err
		}
		updateCid, err := ls.Store(ipld.LinkContext{}, linkProto, unode)
		if err != nil {
			log.Errorw("Failed to store update in link system", "err", err)
			return err
		}
		updCid = updateCid.(cidlink.Link).Cid
		if _, err = tx.ExecContext(ctx, addHeadSQL, updCid.String(), time.Now(), "", LATEST_UPDATE); err != nil {
			return err
		}
		if head != "" {
			if _, err = tx.ExecContext(ctx, updateHeadSQL, PREVIOUS_UPDATE, head); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return cid.Undef, err
	}
	return updCid, nil
}

// drainWorker adds a worker to the list of workers to not give work to.
func (s *stateDB) DrainWorker(ctx context.Context, worker string) error {
	if _, err := s.db().ExecContext(ctx, drainedAddSQL, worker); err != nil {
		return err
	}
	return nil
}

// undrainWorker adds a worker to the list of workers to not give work to.
func (s *stateDB) UndrainWorker(ctx context.Context, worker string) error {
	if _, err := s.db().ExecContext(ctx, drainedDelSQL, worker); err != nil {
		return err
	}
	return nil
}

// ResetWorkerTasks finds all in progress tasks for a worker and resets them to as if they had never been run
func (s *stateDB) ResetWorkerTasks(ctx context.Context, worker string) error {
	var resetTasks []*tasks.Task
	err := s.transact(ctx, func(tx *sql.Tx) error {
		inProgressWorkerTasks, err := tx.QueryContext(ctx, workerTasksByStatusSQL, worker, int64(tasks.InProgress))
		if err != nil {
			return err
		}

		var queriedTasks []*tasks.Task
		for inProgressWorkerTasks.Next() {
			var serialized string
			err := inProgressWorkerTasks.Scan(&serialized)
			if err != nil {
				return err
			}

			tp := tasks.TaskPrototype.NewBuilder()
			if err = dagjson.Decode(tp, bytes.NewBufferString(serialized)); err != nil {
				return err
			}
			task, err := tasks.UnwrapTask(tp.Build())
			if err != nil {
				return err
			}
			queriedTasks = append(queriedTasks, task)
		}

		for _, task := range queriedTasks {
			updatedTask := task.Reset()
			utNode, err := updatedTask.ToNode()
			if err != nil {
				return err
			}
			lnk, data, err := serializeToJSON(ctx, utNode)
			if err != nil {
				return err
			}
			// save the update back to DB
			_, err = tx.ExecContext(ctx, unassignTaskSQL, updatedTask.UUID, data, lnk.String())
			if err != nil {
				return err
			}
			// reset the task in the task status ledger
			_, err = tx.ExecContext(ctx, upsertTaskStatusSQL, updatedTask.UUID, int64(updatedTask.Status), updatedTask.Stage, 0, time.Now())
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

func (s *stateDB) Delete(ctx context.Context, uuid string) error {
	task, _, err := s.getWithTag(ctx, uuid)
	if err != nil {
		return err
	}
	if task == nil {
		return ErrTaskNotFound
	}
	if !(task.HasSchedule() || int64(task.Status) == int64(tasks.Available)) {
		return ErrNoDeleteInProgressTasks
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, deleteTaskSQL, uuid)
		return err
	})
}
