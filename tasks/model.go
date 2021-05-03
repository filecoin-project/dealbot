package tasks

import (
	"errors"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/ipld/go-ipld-prime/schema"
)

// LogStatus is a function that logs messages
type LogStatus func(msg string, keysAndValues ...interface{})

// UpdateStage updates the stage & current stage details for a deal, and
// records the previous stage in the task status ledger as needed
type UpdateStage func(stage string, stageDetails StageDetails) error

// NodeConfig specifies parameters to a running deal bot
type NodeConfig struct {
	DataDir       string
	NodeDataDir   string
	WalletAddress address.Address
}

// TaskEvent logs a change in either status
type TaskEvent struct {
	Status Status
	Stage  string
	At     time.Time
}

var (
	// Available indicates a task is ready to be assigned to a deal bot
	Available Status = &_Status{x: 1}
	// InProgress means the task is running
	InProgress Status = &_Status{x: 2}
	// Successful means the task completed successfully
	Successful Status = &_Status{x: 3}
	// Failed means the task has failed
	Failed Status = &_Status{x: 4}
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

func asStageDetails(description, expected string) StageDetails {
	return &_StageDetails{
		Description:      asStrM(description),
		ExpectedDuration: asStrM(expected),
		Logs:             _List_Logs{[]_Logs{}},
		UpdatedAt:        _Time__Maybe{m: schema.Maybe_Absent},
	}
}

// ConnectivityStages are stages that occur prior to initiating a deal
var ConnectivityStages = map[string]StageDetails{
	"MinerOnline":  asStageDetails("Miner is online", "a few seconds"),
	"QueryAsk":     asStageDetails("Miner responds to query ask", "a few seconds"),
	"CheckPrice":   asStageDetails("Miner meets price criteria", ""),
	"ClientImport": asStageDetails("Importing data into Lotus", "a few minutes"),
}

// RetrievalStages are stages that occur in a retrieval deal
var RetrievalStages = map[string]StageDetails{
	"ProposeRetrieval":  asStageDetails("Send retrieval to miner", ""),
	"DealAccepted":      asStageDetails("Miner accepts deal", "a few seconds"),
	"FirstByteReceived": asStageDetails("First byte of data received from miner", "a few seconds, or several hours when unsealing"),
	"DealComplete":      asStageDetails("All bytes received and deal is completed", "a few seconds"),
}

// AddLog adds a log message to details about the current stage
func AddLog(stageDetails StageDetails, log string) {
	now := time.Now()
	stageDetails.UpdatedAt.m = schema.Maybe_Value
	stageDetails.UpdatedAt.v.x = now.UnixNano()
	stageDetails.Logs.x = append(stageDetails.Logs.x, _Logs{
		Log:       _String{log},
		UpdatedAt: mktime(now),
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
	err := updateStage(stage, stageDetails)
	if err != nil {
		return err
	}
	for _, step := range steps {
		err := step.stepExecution()
		if err != nil {
			return err
		}
		AddLog(stageDetails, step.stepSuccess)
		err = updateStage(stage, stageDetails)
		if err != nil {
			return nil
		}
	}
	return nil
}

func (t *_Time) Time() time.Time {
	return time.Unix(0, t.x)
}
