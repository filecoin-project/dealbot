package tasks

import "github.com/filecoin-project/go-address"

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

type Status int

const (
	Available Status = iota + 1
	InProgress
	Successful
	Failed
)
