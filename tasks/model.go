package tasks

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/filecoin-project/go-address"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/crypto"
)

type LogStatus func(msg string, keysAndValues ...interface{})
type UpdateStage func(stageName string, stage *StageData) error

type NodeConfig struct {
	DataDir       string
	NodeDataDir   string
	WalletAddress address.Address
}

type StageData struct {
	// Human-readable fields.
	Description      string    `json:"description,omitempty"`
	ExpectedDuration string    `json:"expected_duration,omitempty"`
	Logs             []*Log    `json:"logs"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

type Log struct {
	Log       string    `json:"log"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type Task struct {
	UUID          string         `json:"uuid"`
	Status        Status         `json:"status"`
	WorkedBy      string         `json:"worked_by,omitempty"` // which dealbot works on that task
	StageName     string         `json:"stage_name"`
	StageData     *StageData     `json:"stage_data,omitempty"`
	StartedAt     time.Time      `json:"started_at,omitempty"` // the time the task was assigned first assigned to the dealbot
	RetrievalTask *RetrievalTask `json:"retrieval_task,omitempty"`
	StorageTask   *StorageTask   `json:"storage_task,omitempty"`
	Signature     []byte         `json:"signature,omitempty"` // signature of Task with this field set to nil
}

type TaskEvent struct {
	Status    Status
	StageName string
	At        time.Time
}

func (t Task) Bytes() []byte {
	b, err := json.Marshal(&t)
	if err != nil {
		return []byte{}
	}
	return b
}

func (t *Task) Sign(privKey crypto.PrivKey) error {
	var err error
	t.Signature = nil
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

// ConnectivityStages are stages that occur prior to initiating a deal
var ConnectivityStages = map[string]StageData{
	"MinerOnline": {
		Description:      "Miner is online",
		ExpectedDuration: "a few seconds",
	},
	"QueryAsk": {
		Description:      "Miner responds to query ask",
		ExpectedDuration: "a few seconds",
	},
	"CheckPrice": {
		Description:      "Miner meets price criteria",
		ExpectedDuration: "",
	},
	"ClientImport": {
		Description:      "Importing data into Lotus",
		ExpectedDuration: "a few minutes",
	},
}

// RetrievalStages are stages that occur in a retrieval deal
var RetrievalStages = map[string]StageData{
	"ProposeRetrieval": {
		Description:      "Send retrieval to miner",
		ExpectedDuration: "",
	},
	"DealAccepted": {
		Description:      "Miner accepts deal",
		ExpectedDuration: "a few seconds",
	},
	"FirstByteReceived": {
		Description:      "First byte of data received from miner",
		ExpectedDuration: "a few seconds, or several hours when unsealing",
	},
	"DealComplete": {
		Description:      "All bytes received and deal is completed",
		ExpectedDuration: "a few seconds",
	},
}

func AddLog(stageData *StageData, log string) {
	now := time.Now()
	stageData.UpdatedAt = now
	stageData.Logs = append(stageData.Logs, &Log{
		Log:       log,
		UpdatedAt: now,
	})
}

type step struct {
	stepExecution func() error
	stepSuccess   string
}

func executeStage(stageName string, updateStage UpdateStage, steps []step) error {
	stageData, ok := ConnectivityStages[stageName]
	if !ok {
		return errors.New("unknown stage")
	}
	err := updateStage(stageName, &stageData)
	if err != nil {
		return err
	}
	for _, step := range steps {
		err := step.stepExecution()
		if err != nil {
			return err
		}
		AddLog(&stageData, step.stepSuccess)
		err = updateStage(stageName, &stageData)
		if err != nil {
			return nil
		}
	}
	return nil
}
