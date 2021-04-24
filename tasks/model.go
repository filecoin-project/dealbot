package tasks

import (
	"encoding/json"
	"time"

	"github.com/filecoin-project/go-address"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/crypto"
)

type UpdateStatus func(msg string, keysAndValues ...interface{})

type NodeConfig struct {
	DataDir       string
	NodeDataDir   string
	WalletAddress address.Address
}

type Task struct {
	UUID          string         `json:"uuid"`
	Status        Status         `json:"status"`
	WorkedBy      string         `json:"worked_by,omitempty"`  // which dealbot works on that task
	StartedAt     time.Time      `json:"started_at,omitempty"` // the time the task was assigned first assigned to the dealbot
	RetrievalTask *RetrievalTask `json:"retrieval_task,omitempty"`
	StorageTask   *StorageTask   `json:"storage_task,omitempty"`
}

type TaskEvent struct {
	Status Status
	At     time.Time
}

func (t Task) Bytes() []byte {
	b, err := json.Marshal(&t)
	if err != nil {
		return []byte{}
	}
	return b
}

type AuthenticatedTask struct {
	Task
	Signature []byte
}

func (t *AuthenticatedTask) Sign(privKey crypto.PrivKey) error {
	var err error
	t.Signature, err = privKey.Sign(t.Bytes())
	return err
}

func (t *Task) Log(log *logging.ZapEventLogger) {
	if t.RetrievalTask != nil {
		log.Infow("retrieval task", "uuid", t.UUID, "status", t.Status, "worked_by", t.WorkedBy)
	} else if t.StorageTask != nil {
		log.Infow("storage task", "uuid", t.UUID, "status", t.Status, "worked_by", t.WorkedBy)
	} else {
		panic("both tasks are nil")
	}
}

type Status int

const (
	Available Status = iota + 1
	InProgress
	Successful
	Failed
)

var statusNames = map[Status]string{
	Available:  "Available",
	InProgress: "InProgress",
	Successful: "Successful",
	Failed:     "Failed",
}

func (s Status) String() string {
	return statusNames[s]
}
