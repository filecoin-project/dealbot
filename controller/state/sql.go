package state

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/metrics"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/google/uuid"
	logging "github.com/ipfs/go-log/v2"
	crypto "github.com/libp2p/go-libp2p-crypto"

	// include sqlite driver
	_ "modernc.org/sqlite"
	// include postgres driver
	_ "github.com/lib/pq"
)

var log = logging.Logger("controller-state")

func NewSql(driver, conn string, identity crypto.PrivKey) (State, error) {
	db, err := sql.Open(driver, conn)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tasks (
		uuid text,
		ts timestamp,
		data text,
		PRIMARY KEY(uuid,ts)
		)`)
	if err != nil {
		return nil, err
	}

	var cnt int
	if err := db.QueryRow("SELECT COUNT(*) from tasks").Scan(&cnt); err != nil {
		return nil, err
	}

	if cnt == 0 {
		initTasks := []*tasks.AuthenticatedTask{
			{
				tasks.Task{
					UUID:   uuid.New().String()[:8],
					Status: tasks.Available,
					RetrievalTask: &tasks.RetrievalTask{
						Miner:      "t01000",
						PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
						CARExport:  false,
					},
				}, []byte{},
			},
			{
				tasks.Task{
					UUID:   uuid.New().String()[:8],
					Status: tasks.Available,
					RetrievalTask: &tasks.RetrievalTask{
						Miner:      "t01000",
						PayloadCID: "bafk2bzacecettil4umy443e4ferok7jbxiqqseef7soa3ntelflf3zkvvndbg",
						CARExport:  false,
					},
				}, []byte{},
			},
			{
				tasks.Task{
					UUID:   uuid.New().String()[:8],
					Status: tasks.Available,
					RetrievalTask: &tasks.RetrievalTask{
						Miner:      "f0127896",
						PayloadCID: "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
						CARExport:  false,
					},
				}, []byte{},
			},
			{
				tasks.Task{
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
				}, []byte{},
			},
		}
		db := &sqlDB{db, identity}
		for _, t := range initTasks {
			db.save(t)
		}
	}

	return &sqlDB{db, identity}, nil
}

type sqlDB struct {
	*sql.DB
	crypto.PrivKey
}

func (s *sqlDB) Get(UUID string) (*tasks.AuthenticatedTask, error) {
	var serialized string
	err := s.DB.QueryRow("SELECT data FROM tasks WHERE uuid=? ORDER BY ts DESC limit 1", UUID).Scan(&serialized)
	if err != nil {
		return nil, err
	}
	var task tasks.AuthenticatedTask
	if err := json.Unmarshal([]byte(serialized), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *sqlDB) GetAll() ([]*tasks.AuthenticatedTask, error) {
	tasklist := make([]*tasks.AuthenticatedTask, 0)
	var serialized string
	rows, err := s.DB.Query("SELECT data FROM tasks t1 where ts=(select MAX(ts) from tasks t2 WHERE t1.uuid = t2.uuid)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&serialized); err != nil {
			return nil, err
		}
		var task tasks.AuthenticatedTask
		if err := json.Unmarshal([]byte(serialized), &task); err != nil {
			return nil, err
		}
		tasklist = append(tasklist, &task)
	}
	return tasklist, nil

}

func (s *sqlDB) Update(UUID string, req *client.UpdateTaskRequest, recorder metrics.MetricsRecorder) (*tasks.AuthenticatedTask, error) {
	latest, err := s.Get(UUID)
	if err != nil {
		return nil, err
	}

	if latest.Status == tasks.Available {
		latest.WorkedBy = req.WorkedBy
		latest.StartedAt = time.Now()
	} else {
		if latest.WorkedBy != req.WorkedBy {
			return nil, errors.New("task already acquired")
		}
	}
	log.Infow("state update", "uuid", latest.UUID, "status", req.Status, "worked_by", req.WorkedBy)

	latest.Status = req.Status
	latest.Signature, err = s.PrivKey.Sign(latest.Bytes())
	if err != nil {
		return nil, err
	}

	//save the update back to DB
	if err := s.save(latest); err != nil {
		return nil, err
	}

	if err := recorder.ObserveTask(latest); err != nil {
		return nil, err
	}

	return latest, nil

}

func (s *sqlDB) save(t *tasks.AuthenticatedTask) error {
	//save the update back to DB
	bytes, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if res, err := s.DB.Exec("INSERT INTO tasks (uuid, ts, data) VALUES($1,$2,$3)", t.UUID, time.Now(), bytes); err != nil {
		return err
	} else if rows, err := res.RowsAffected(); err != nil || rows != 1 {
		return err
	}
	return nil
}

func (s *sqlDB) NewStorageTask(storageTask *tasks.StorageTask) (*tasks.AuthenticatedTask, error) {
	task := &tasks.AuthenticatedTask{
		tasks.Task{
			UUID:        uuid.New().String()[:8],
			Status:      tasks.Available,
			StorageTask: storageTask,
		},
		[]byte{},
	}
	var err error
	task.Signature, err = s.PrivKey.Sign(task.Bytes())
	if err != nil {
		return nil, err
	}

	//save the update back to DB
	if err := s.save(task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *sqlDB) NewRetrievalTask(retrievalTask *tasks.RetrievalTask) (*tasks.AuthenticatedTask, error) {
	task := &tasks.AuthenticatedTask{
		tasks.Task{
			UUID:          uuid.New().String()[:8],
			Status:        tasks.Available,
			RetrievalTask: retrievalTask,
		},
		[]byte{},
	}
	var err error
	task.Signature, err = s.PrivKey.Sign(task.Bytes())
	if err != nil {
		return nil, err
	}

	//save the update back to DB
	if err := s.save(task); err != nil {
		return nil, err
	}

	return task, nil

}
