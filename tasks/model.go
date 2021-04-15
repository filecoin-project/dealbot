package tasks

import (
	"github.com/filecoin-project/go-address"
	logging "github.com/ipfs/go-log/v2"
)

type UpdateStatus func(msg string, keysAndValues ...interface{})

type NodeConfig struct {
	DataDir       string
	NodeDataDir   string
	WalletAddress address.Address
}

type Task struct {
	UUID     string `json:"uuid"`
	Status   Status `json:"status"`
	WorkedBy string `json:"worked_by,omitempty"` // which dealbot works on that task

	RetrievalTask *RetrievalTask `json:"retrieval_task,omitempty"`
	StorageTask   *StorageTask   `json:"storage_task,omitempty"`
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

var StatusNames = map[Status]string{
	Available:  "Available",
	InProgress: "InProgress",
	Successful: "Successful",
	Failed:     "Failed",
}
