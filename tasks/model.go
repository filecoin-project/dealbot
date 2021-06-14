package tasks

//go:generate go run gen.go .

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	linksystem "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/multiformats/go-multicodec"

	// Require graphql generation here so that it is included in go.mod and available for go:generate above.
	_ "github.com/ipld/go-ipld-graphql/gen"
)

// LogStatus is a function that logs messages
type LogStatus func(msg string, keysAndValues ...interface{})

// UpdateStage updates the stage & current stage details for a deal, and
// records the previous stage in the task status ledger as needed
type UpdateStage func(ctx context.Context, stage string, stageDetails StageDetails) error

// NodeConfig specifies parameters to a running deal bot
type NodeConfig struct {
	DataDir          string
	NodeDataDir      string
	WalletAddress    address.Address
	MinWalletBalance big.Int
	MinWalletCap     big.Int
	PostHook         string
}

// TaskEvent logs a change in either status
type TaskEvent struct {
	Status Status
	Stage  string
	Run    int
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

// staticStageDetails returns a func,
// so that we record each time the stage is seen,
// given that StageDetails.Of uses time.Now.
func staticStageDetails(description, expected string) func() StageDetails {
	return func() StageDetails {
		return Type.StageDetails.Of(description, expected)
	}
}

// BlankStage is a fallback stage for deals that fail early in an unknown stage
var BlankStage = staticStageDetails("Unknown stage", "")

// CommonStages are stages near the beginning of a deal shared between storage & retrieval
var CommonStages = map[string]func() StageDetails{
	"MinerOnline":  staticStageDetails("Miner is online", "a few seconds"),
	"QueryAsk":     staticStageDetails("Miner responds to query ask", "a few seconds"),
	"CheckPrice":   staticStageDetails("Miner meets price criteria", ""),
	"ClientImport": staticStageDetails("Importing data into Lotus", "a few minutes"),
	"ProposeDeal":  staticStageDetails("Send proposal to miner", ""),
}

// RetrievalStages are stages that occur in a retrieval deal
var RetrievalStages = map[string]func() StageDetails{
	"DealAccepted":      staticStageDetails("Miner accepts deal", "a few seconds"),
	"FirstByteReceived": staticStageDetails("First byte of data received from miner", "a few seconds, or several hours when unsealing"),
	"AllBytesReceived":  staticStageDetails("All bytes received, deal wrapping up", "a few seconds"),
	"DealComplete":      staticStageDetails("Deal is complete", "a few seconds"),
}

// the multi-codec and hash we use for cid links by default
var linkProto = linksystem.LinkBuilder{Prefix: cid.Prefix{
	Version:  1,
	Codec:    uint64(multicodec.DagJson),
	MhType:   uint64(multicodec.Sha2_256),
	MhLength: 32,
}}

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

type logFunc func(string)

type step struct {
	stepExecution func(logFunc) error
	stepSuccess   string
}

func executeStage(ctx context.Context, stage string, updateStage UpdateStage, steps []step) error {
	stageDetailsFn := CommonStages[stage]
	if stageDetailsFn == nil {
		return errors.New("unknown stage")
	}
	stageDetails := stageDetailsFn()
	err := updateStage(ctx, stage, stageDetails)
	if err != nil {
		return err
	}
	for _, step := range steps {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		lf := func(l string) {
			stageDetails = AddLog(stageDetails, l)
		}

		err = step.stepExecution(lf)
		if err != nil {
			return err
		}
		stageDetails = AddLog(stageDetails, step.stepSuccess)
		err = updateStage(ctx, stage, stageDetails)
		if err != nil {
			return err
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

func (t *_Task) Finalize(ctx context.Context, s ipld.Storer) (FinishedTask, error) {
	if t.Status != *Failed && t.Status != *Successful {
		return nil, fmt.Errorf("task cannot be finalized as it is not in a finished state")
	}

	logs := parseFinalLogs(t)
	ft := _FinishedTask{
		Status:             t.Status,
		StartedAt:          *t.StartedAt.v,
		ErrorMessage:       t.ErrorMessage,
		RetrievalTask:      t.RetrievalTask,
		StorageTask:        t.StorageTask,
		DealID:             _Int{int64(logs.dealID)},
		MinerMultiAddr:     _String{logs.minerAddr},
		ClientApparentAddr: _String{logs.clientAddr},
		MinerLatencyMS:     logs.minerLatency,
		TimeToFirstByteMS:  logs.timeFirstByte,
		TimeToLastByteMS:   logs.timeLastByte,
		MinerVersion:       logs.minerVersion,
		ClientVersion:      logs.clientVersion,
		Size:               logs.size,
		PayloadCID:         logs.payloadCID,
	}
	// events to dag item
	logList := &_List_StageDetails{}
	if t.PastStageDetails.Exists() {
		logList = t.PastStageDetails.Must()
	}
	logLnk, err := linkProto.Build(ctx, ipld.LinkContext{}, logList, s)
	if err != nil {
		return nil, err
	}
	ft.Events = _Link_List_StageDetails{logLnk}

	return &ft, nil
}

type logExtraction struct {
	dealID        int
	minerAddr     string
	minerVersion  _String__Maybe
	clientAddr    string
	clientVersion _String__Maybe
	minerLatency  _Int__Maybe
	timeFirstByte _Int__Maybe
	timeLastByte  _Int__Maybe
	size          _Int__Maybe
	payloadCID    _String__Maybe
}

func parseFinalLogs(t Task) *logExtraction {
	le := &logExtraction{
		minerVersion:  _String__Maybe{m: schema.Maybe_Absent},
		minerLatency:  _Int__Maybe{m: schema.Maybe_Absent},
		timeFirstByte: _Int__Maybe{m: schema.Maybe_Absent},
		timeLastByte:  _Int__Maybe{m: schema.Maybe_Absent},
	}

	if t.StorageTask.Exists() {
		le.size = _Int__Maybe{m: schema.Maybe_Value, v: &t.StorageTask.Must().Size}
	} else if t.RetrievalTask.Exists() {
		le.payloadCID = _String__Maybe{m: schema.Maybe_Value, v: &t.RetrievalTask.Must().PayloadCID}
	}

	// If the task failed early, we might not have some of the info.
	if !t.StartedAt.Exists() || !t.PastStageDetails.Exists() {
		return le
	}

	// First/last byte stage substrings for tasks.
	// They get matched

	// If the provider is verifying the data, it has all the bytes.
	firstByteSubstring := "opening data transfer"
	lastByteSubstring := "provider is verifying the data"

	// First/last byte log entries for retrieval tasks.
	// From RetrievalStages above; matches stage descriptions.
	if t.RetrievalTask.Exists() {
		firstByteSubstring = "First byte of data received"
		lastByteSubstring = "All bytes received"
	}

	start := t.StartedAt.Must().Time()
	for _, stage := range t.PastStageDetails.Must().x {

		// First/last byte deriving also looks at stage descriptions.
		if stage.UpdatedAt.Exists() && stage.Description.Exists() {
			entry := stage.Description.Must().String()
			entryTime := stage.UpdatedAt.Must().Time()
			if !le.timeFirstByte.Exists() && strings.Contains(entry, firstByteSubstring) {
				le.timeFirstByte.m = schema.Maybe_Value
				le.timeFirstByte.v = &_Int{entryTime.Sub(start).Milliseconds()}
			} else if !le.timeLastByte.Exists() && strings.Contains(entry, lastByteSubstring) {
				le.timeLastByte.m = schema.Maybe_Value
				le.timeLastByte.v = &_Int{entryTime.Sub(start).Milliseconds()}
			}
		}

		for _, log := range stage.Logs.x {
			entry := log.Log.String()
			entryTime := log.UpdatedAt.Time()

			if !le.timeFirstByte.Exists() && strings.Contains(entry, firstByteSubstring) {
				le.timeFirstByte.m = schema.Maybe_Value
				le.timeFirstByte.v = &_Int{entryTime.Sub(start).Milliseconds()}
			} else if !le.timeLastByte.Exists() && strings.Contains(entry, lastByteSubstring) {
				le.timeLastByte.m = schema.Maybe_Value
				le.timeLastByte.v = &_Int{entryTime.Sub(start).Milliseconds()}
			}

			if !le.minerVersion.Exists() && strings.Contains(entry, "NetAgentVersion: ") {
				le.minerVersion.m = schema.Maybe_Value
				le.minerVersion.v = &_String{strings.TrimPrefix(entry, "NetAgentVersion: ")}
			}
			if !le.clientVersion.Exists() && strings.Contains(entry, "ClientVersion: ") {
				le.clientVersion.m = schema.Maybe_Value
				le.clientVersion.v = &_String{strings.TrimPrefix(entry, "ClientVersion: ")}
			}
			if le.minerAddr == "" && strings.Contains(entry, "RemotePeerAddr: ") {
				le.minerAddr = strings.TrimPrefix(entry, "RemotePeerAddr: ")
			}
			if !le.minerLatency.Exists() && strings.Contains(entry, "RemotePeerLatency: ") {
				le.minerLatency.m = schema.Maybe_Value
				val, err := strconv.Atoi(strings.TrimPrefix(entry, "RemotePeerLatency: "))
				if err == nil {
					le.minerLatency.v = &_Int{int64(val)}
				}
			}
			if le.size.IsAbsent() && strings.Contains(entry, "bytes received:") {
				le.size.m = schema.Maybe_Value
				b, err := strconv.ParseInt(strings.TrimPrefix(entry, "bytes received: "), 10, 10)
				if err == nil {
					le.size.v = &_Int{b}
				}
			}
			if le.payloadCID.IsAbsent() && strings.Contains(entry, "PayloadCID:") {
				le.payloadCID.m = schema.Maybe_Value
				le.payloadCID.v = &_String{strings.TrimPrefix(entry, "PayloadCID: ")}
			}
		}
	}

	// Sometimes we'll see an event for the first byte and no event for the
	// last byte, such as when doing a retrieval task and the data is
	// already present locally.
	// In those casese, assume that the transfer was immediate.
	if le.timeFirstByte.Exists() && !le.timeLastByte.Exists() {
		le.timeLastByte = le.timeFirstByte
	}

	return le
}

func (t *_Task) UpdateTask(tsk UpdateTask) (Task, error) {
	stage := ""
	if tsk.Stage.Exists() {
		stage = tsk.Stage.Must().String()
	}
	updatedTask := _Task{
		UUID:                t.UUID,
		Status:              tsk.Status,
		WorkedBy:            _String__Maybe{m: schema.Maybe_Value, v: &tsk.WorkedBy},
		ErrorMessage:        tsk.ErrorMessage,
		Stage:               _String{stage},
		StartedAt:           t.StartedAt,
		RunCount:            tsk.RunCount,
		CurrentStageDetails: tsk.CurrentStageDetails,
		RetrievalTask:       t.RetrievalTask,
		StorageTask:         t.StorageTask,
	}

	// On stage transitions, archive the current stage.
	if stage != t.Stage.x && t.CurrentStageDetails.Exists() {
		if !t.PastStageDetails.Exists() {
			updatedTask.PastStageDetails = _List_StageDetails__Maybe{m: schema.Maybe_Value, v: &_List_StageDetails{x: []_StageDetails{*t.CurrentStageDetails.v}}}
		} else {
			updatedTask.PastStageDetails = _List_StageDetails__Maybe{m: schema.Maybe_Value, v: &_List_StageDetails{x: append(t.PastStageDetails.v.x, *t.CurrentStageDetails.v)}}
		}
	} else {
		updatedTask.PastStageDetails = t.PastStageDetails
	}

	return &updatedTask, nil
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

func (arp *_AuthenticatedRecord__Prototype) Of(c cid.Cid, sig []byte) *_AuthenticatedRecord {
	ar := _AuthenticatedRecord{
		Record:    _Link_FinishedTask{x: linksystem.Link{Cid: c}},
		Signature: _Bytes{x: sig},
	}
	return &ar
}

func (alp *_List_AuthenticatedRecord__Prototype) Of(ars []*_AuthenticatedRecord) *_List_AuthenticatedRecord {
	al := _List_AuthenticatedRecord{
		x: []_AuthenticatedRecord{},
	}
	for _, a := range ars {
		al.x = append(al.x, *a)
	}
	return &al
}

func (rup *_RecordUpdate__Prototype) Of(rcrds *_List_AuthenticatedRecord, previous cid.Cid, previousSig []byte) *_RecordUpdate {
	lrm := _Link__Maybe{m: schema.Maybe_Null, v: nil}
	if previous != cid.Undef {
		lrm.m = schema.Maybe_Value
		lrm.v = &_Link{x: linksystem.Link{Cid: previous}}
	}
	ru := _RecordUpdate{
		Records:  *rcrds,
		SigPrev:  _Bytes{x: previousSig},
		Previous: lrm,
	}
	return &ru
}

func (tl *_FinishedTasks__Prototype) Of(ts []FinishedTask) *_FinishedTasks {
	t := _FinishedTasks{
		x: make([]_FinishedTask, 0, len(ts)),
	}
	for _, c := range ts {
		t.x = append(t.x, *c)
	}
	return &t
}
