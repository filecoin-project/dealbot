package tasks

//go:generate go run gen.go .

import (
	"errors"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/google/uuid"
	"github.com/ipld/go-ipld-prime/schema"

	// Require graphql generation here so that it is included in go.mod and available for go:generate above.
	_ "github.com/ipld/go-ipld-graphql/gen"
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

func (f *_Status__Prototype) Of(x int) Status {
	switch x {
	case 1:
		return Available
	case 2:
		return InProgress
	case 3:
		return Successful
	case 4:
		return Failed
	default:
		return nil
	}
}

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
func AddLog(stageDetails StageDetails, log string) StageDetails {
	now := time.Now()

	logs := make([]_Logs, 0)
	if stageDetails != nil && stageDetails.Logs.x != nil {
		logs = append(logs, stageDetails.Logs.x...)
	}
	logs = append(logs, _Logs{
		Log:       _String{log},
		UpdatedAt: mktime(now),
	})
	n := _StageDetails{
		Description:      stageDetails.Description,
		ExpectedDuration: stageDetails.ExpectedDuration,
		UpdatedAt:        _Time__Maybe{m: schema.Maybe_Value, v: &_Time{now.UnixNano()}},
		Logs: _List_Logs{
			x: logs,
		},
	}

	return &n
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
		stageDetails = AddLog(stageDetails, step.stepSuccess)
		err = updateStage(stage, stageDetails)
		if err != nil {
			return nil
		}
	}
	return nil
}

func (sdp *_StageDetails__Prototype) Of(desc, expected string) *_StageDetails {
	sd := _StageDetails{
		Description:      _String__Maybe{m: schema.Maybe_Value, v: &_String{desc}},
		ExpectedDuration: _String__Maybe{m: schema.Maybe_Value, v: &_String{expected}},
		Logs:             _List_Logs{[]_Logs{}},
		UpdatedAt:        _Time__Maybe{m: schema.Maybe_Value, v: &_Time{x: time.Now().UnixNano()}},
	}
	return &sd
}

// WithLog makes a copy of the stage details with an additional log appended.
func (sd *_StageDetails) WithLog(log string) *_StageDetails {
	nl := _List_Logs{
		x: append(sd.Logs.x, _Logs{
			Log:       _String{log},
			UpdatedAt: _Time{x: time.Now().UnixNano()},
		}),
	}
	n := _StageDetails{
		Description:      sd.Description,
		ExpectedDuration: sd.ExpectedDuration,
		Logs:             nl,
		UpdatedAt:        _Time__Maybe{m: schema.Maybe_Value, v: &_Time{x: time.Now().UnixNano()}},
	}
	return &n
}

func (t *_Time) Time() time.Time {
	return time.Unix(0, t.x)
}

func (tp *_Task__Prototype) New(r RetrievalTask, s StorageTask) Task {
	t := _Task{
		UUID:                _String{uuid.New().String()},
		Status:              *Available,
		WorkedBy:            _String__Maybe{m: schema.Maybe_Value, v: &_String{""}},
		Stage:               _String{""},
		CurrentStageDetails: _StageDetails__Maybe{m: schema.Maybe_Absent},
		StartedAt:           _Time__Maybe{m: schema.Maybe_Absent},
		RetrievalTask:       _RetrievalTask__Maybe{m: schema.Maybe_Absent},
		StorageTask:         _StorageTask__Maybe{m: schema.Maybe_Absent},
	}
	if r != nil {
		t.RetrievalTask.m = schema.Maybe_Value
		t.RetrievalTask.v = r
	}
	if s != nil {
		t.StorageTask.m = schema.Maybe_Value
		t.StorageTask.v = s
	}
	return &t
}

func (t *_Task) Assign(worker string, status Status) Task {
	newTask := _Task{
		UUID:                t.UUID,
		Status:              *status,
		WorkedBy:            _String__Maybe{m: schema.Maybe_Value, v: &_String{worker}},
		Stage:               t.Stage,
		CurrentStageDetails: t.CurrentStageDetails,
		PastStageDetails:    t.PastStageDetails,
		StartedAt:           _Time__Maybe{m: schema.Maybe_Value, v: &_Time{x: time.Now().UnixNano()}},
		RetrievalTask:       t.RetrievalTask,
		StorageTask:         t.StorageTask,
	}

	//todo: sign
	return &newTask
}

func (t *_Task) Update(status Status, stage string, details StageDetails) (Task, error) {
	updatedTask := _Task{
		UUID:          t.UUID,
		Status:        *status,
		WorkedBy:      t.WorkedBy,
		Stage:         _String{stage},
		StartedAt:     t.StartedAt,
		RetrievalTask: t.RetrievalTask,
		StorageTask:   t.StorageTask,
	}

	// On stage transitions, archive the current stage.
	if stage != t.Stage.x && t.CurrentStageDetails.Exists() {
		if !t.PastStageDetails.Exists() {
			t.PastStageDetails = _List_StageDetails__Maybe{m: schema.Maybe_Value, v: &_List_StageDetails{x: []_StageDetails{*t.CurrentStageDetails.v}}}
		} else {
			t.PastStageDetails.v.x = append(t.PastStageDetails.v.x, *t.CurrentStageDetails.v)
		}
	}

	if details == nil {
		t.CurrentStageDetails = _StageDetails__Maybe{m: schema.Maybe_Absent}
	} else {
		t.CurrentStageDetails = _StageDetails__Maybe{m: schema.Maybe_Value, v: details}
	}

	//todo: sign
	return &updatedTask, nil
}

func (t *_Task) UpdateTask(tsk UpdateTask) (Task, error) {
	stage := ""
	if tsk.Stage.Exists() {
		stage = tsk.Stage.Must().x
	}
	nt, err := t.Update(&tsk.Status, stage, tsk.CurrentStageDetails.v)
	if err != nil {
		return nil, err
	}
	nt.WorkedBy = _String__Maybe{m: schema.Maybe_Value, v: &tsk.WorkedBy}
	//todo: sign
	return nt, nil
}

func (t *_Task) GetUUID() string {
	return t.UUID.x
}

func (tl *_Tasks__Prototype) Of(ts []Task) *_Tasks {
	t := _Tasks{
		x: []_Task{},
	}
	for _, c := range ts {
		t.x = append(t.x, *c)
	}
	return &t
}

func (ts *_Tasks) List() []Task {
	itmsp := make([]_Task, len(ts.x))
	itms := make([]Task, len(ts.x))
	for i := range ts.x {
		itmsp[i] = ts.x[i]
		itms[i] = &itmsp[i]
	}
	return itms
}
