package tasks

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/filecoin-project/go-address"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/crypto"
)

// LogStatus is a function that logs messages
type LogStatus func(msg string, keysAndValues ...interface{})

// UpdateStage updates the stage & current stage details for a deal, and
// records the previous stage in the task status ledger as needed
type UpdateStage func(stage string, stageDetails *StageDetails) error

// NodeConfig specifies parameters to a running deal bot
type NodeConfig struct {
	DataDir       string
	NodeDataDir   string
	WalletAddress address.Address
}

// Task is a task to be run by the deal bot
type Task struct {
	// UUID is a unique identifier for this task
	UUID string `json:"uuid"`
	// Status is the global task status, shared among all tasks types -- always one of Available, InProgress, Successful, or Failure
	Status Status `json:"status"`
	// WorkedBy indicates the dealbot assigned to this task
	WorkedBy string `json:"worked_by,omitempty"`
	// Stage is a more detailed identifier for what part of the process the deal is in. Some stages are unique to the task type
	// When a task status is Successful or Failure, Stage indicates the final stage of the deal making process that was reached
	Stage string `json:"stage"`
	// CurrentStageDeatails offers more information about what is happening in the current stage of the deal making process
	CurrentStageDetails *StageDetails `json:"current_stage_details,omitempty"`
	// StartedAt the time the task was assigned first assigned to a dealbot
	StartedAt time.Time `json:"started_at,omitempty"`
	// RetrievalTask is subparameters for a retrieval deal -- will be nil for a storage deal
	RetrievalTask *RetrievalTask `json:"retrieval_task,omitempty"`
	// StorageTask is subparameters for a storage deal -- will be nil for a retrieval deal
	StorageTask *StorageTask `json:"storage_task,omitempty"`
	// Signature is the crytographic signature of all the data in this task absent the signature field itself (i.e. sig set to nil)
	Signature []byte `json:"signature,omitempty"`
}

// StageDetails offers detailed information about progress within the current stage
type StageDetails struct {
	// Human-readable fields.
	Description      string    `json:"description,omitempty"`
	ExpectedDuration string    `json:"expected_duration,omitempty"`
	Logs             []*Log    `json:"logs"`
	UpdatedAt        time.Time `json:"updated_at,omitempty"`
}

// Log is a message about something that happened in the current stage
type Log struct {
	Log       string    `json:"log"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// TaskEvent logs a change in either status
type TaskEvent struct {
	Status Status
	Stage  string
	At     time.Time
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

// Status is a global task status, shared among all tasks types
type Status int

const (
	// Available indicates a task is ready to be assigned to a deal bot
	Available Status = iota + 1
	// InProgress means the task is running
	InProgress
	// Successful means the task completed successfully
	Successful
	// Failed means the task has failed
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
var ConnectivityStages = map[string]StageDetails{
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
var RetrievalStages = map[string]StageDetails{
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

// AddLog adds a log message to details about the current stage
func AddLog(stageDetails *StageDetails, log string) {
	now := time.Now()
	stageDetails.UpdatedAt = now
	stageDetails.Logs = append(stageDetails.Logs, &Log{
		Log:       log,
		UpdatedAt: now,
	})
}

type step struct {
	stepExecution func() error
	stepSuccess   string
}

func executeStage(stage string, updateStage UpdateStage, steps []step) error {
	stageDetails, ok := ConnectivityStages[stage]
	if !ok {
		return errors.New("unknown stage")
	}
	err := updateStage(stage, &stageDetails)
	if err != nil {
		return err
	}
	for _, step := range steps {
		err := step.stepExecution()
		if err != nil {
			return err
		}
		AddLog(&stageDetails, step.stepSuccess)
		err = updateStage(stage, &stageDetails)
		if err != nil {
			return nil
		}
	}
	return nil
}
