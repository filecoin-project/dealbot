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
	"github.com/multiformats/go-multicodec"

	// Require graphql generation here so that it is included in go.mod and available for go:generate above.
	_ "github.com/ipld/go-ipld-graphql/gen"
)

// LogStatus is a function that logs messages
type LogStatus func(msg string, keysAndValues ...interface{})

// UpdateStage updates the stage & current stage details for a deal, and
// records the previous stage in the task status ledger as needed
type UpdateStage func(ctx context.Context, stage string, stageDetails *StageDetails) error

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
	Available Status = 1
	// InProgress means the task is running
	InProgress Status = 2
	// Successful means the task completed successfully
	Successful Status = 3
	// Failed means the task has failed
	Failed Status = 4
)

//func (f *_Status__Prototype) Of(x int) Status {
//	switch x {
//	case 1:
//		return Available
//	case 2:
//		return InProgress
//	case 3:
//		return Successful
//	case 4:
//		return Failed
//	default:
//		return nil
//	}
//}

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
func staticStageDetails(description, expected string) func() *StageDetails {
	return func() *StageDetails {
		t := Time(time.Now().UnixNano())
		return &StageDetails{
			Description:      &description,
			ExpectedDuration: &expected,
			Logs:             make([]Logs, 0),
			UpdatedAt:        &t,
		}
	}
}

// BlankStage is a fallback stage for deals that fail early in an unknown stage
var BlankStage = staticStageDetails("Unknown stage", "")

// CommonStages are stages near the beginning of a deal shared between storage & retrieval
var CommonStages = map[string]func() *StageDetails{
	"MinerOnline":  staticStageDetails("Miner is online", "a few seconds"),
	"QueryAsk":     staticStageDetails("Miner responds to query ask", "a few seconds"),
	"CheckPrice":   staticStageDetails("Miner meets price criteria", ""),
	"ClientImport": staticStageDetails("Importing data into Lotus", "a few minutes"),
	"ProposeDeal":  staticStageDetails("Send proposal to miner", ""),
}

// RetrievalStages are stages that occur in a retrieval deal
var RetrievalStages = map[string]func() *StageDetails{
	"DealAccepted":      staticStageDetails("Miner accepts deal", "a few seconds"),
	"FirstByteReceived": staticStageDetails("First byte of data received from miner", "a few seconds, or several hours when unsealing"),
	"AllBytesReceived":  staticStageDetails("All bytes received, deal wrapping up", "a few seconds"),
	"DealComplete":      staticStageDetails("Deal is complete", "a few seconds"),
}

// the multi-codec and hash we use for cid links by default
var linkProto = linksystem.LinkPrototype{Prefix: cid.Prefix{
	Version:  1,
	Codec:    uint64(multicodec.DagJson),
	MhType:   uint64(multicodec.Sha2_256),
	MhLength: 32,
}}

// AddLog adds a log message to details about the current stage
func AddLog(stageDetails *StageDetails, log string) *StageDetails {
	now := time.Now()

	logs := make([]Logs, 0)
	if stageDetails != nil && stageDetails.Logs != nil {
		logs = append(logs, stageDetails.Logs...)
	}
	logs = append(logs, Logs{
		Log:       log,
		UpdatedAt: mktime(now),
	})
	t := Time(time.Now().UnixNano())
	n := &StageDetails{
		Description:      stageDetails.Description,
		ExpectedDuration: stageDetails.ExpectedDuration,
		UpdatedAt:        &t,
		Logs:             logs,
	}

	return n
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

func NewStageDetails(desc, expected string) *StageDetails {
	t := Time(time.Now().UnixNano())
	s := &StageDetails{
		Description:      &desc,
		ExpectedDuration: &expected,
		Logs:             make([]Logs, 0),
		UpdatedAt:        &t,
	}
	return s
}

//func (sd *StageDetails) Of(desc, expected string) *StageDetails {
//	s := &StageDetails{
//		Description:      desc,
//		ExpectedDuration: expected,
//		Logs:             make([]*Logs, 0),
//		UpdatedAt:        time.Now().UnixNano(),
//	}
//	return s
//}

// WithLog makes a copy of the stage details with an additional log appended.
func (sd *StageDetails) WithLog(log string) *StageDetails {
	nl := append(sd.Logs, Logs{
		Log:       log,
		UpdatedAt: Time(time.Now().UnixNano()),
	})
	t := Time(time.Now().UnixNano())
	n := &StageDetails{
		Description:      sd.Description,
		ExpectedDuration: sd.ExpectedDuration,
		Logs:             nl,
		UpdatedAt:        &t,
	}
	return n
}

func (t *Time) Time() time.Time {
	return time.Unix(0, int64(*t))
}

//func (tp *_Task__Prototype) New(r RetrievalTask, s StorageTask) Task {
//	t := _Task{
//		UUID:                _String{uuid.New().String()},
//		Status:              *Available,
//		WorkedBy:            _String__Maybe{m: schema.Maybe_Value, v: _String{""}},
//		Stage:               _String{""},
//		CurrentStageDetails: _StageDetails__Maybe{m: schema.Maybe_Absent},
//		StartedAt:           _Time__Maybe{m: schema.Maybe_Absent},
//		RetrievalTask:       _RetrievalTask__Maybe{m: schema.Maybe_Absent},
//		StorageTask:         _StorageTask__Maybe{m: schema.Maybe_Absent},
//	}
//	if r != nil {
//		t.RetrievalTask.m = schema.Maybe_Value
//		t.RetrievalTask.v = r
//	}
//	if s != nil {
//		t.StorageTask.m = schema.Maybe_Value
//		t.StorageTask.v = s
//	}
//	return &t
//}

func NewTask(r *RetrievalTask, s *StorageTask) *Task {
	emptyStr := ""
	t := Task{
		UUID:                uuid.New().String(),
		Status:              Available,
		WorkedBy:            &emptyStr,
		Stage:               "",
		CurrentStageDetails: nil,
		StartedAt:           nil,
		RetrievalTask:       nil,
		StorageTask:         nil,
	}
	if r != nil {
		t.RetrievalTask = r
	}
	if s != nil {
		t.StorageTask = s
	}
	return &t
}

func (t *Task) Reset() *Task {
	emptyStr := ""
	newTask := Task{
		UUID:          t.UUID,
		Status:        Available,
		WorkedBy:      &emptyStr,
		Stage:         "",
		RetrievalTask: t.RetrievalTask,
		StorageTask:   t.StorageTask,
	}
	return &newTask
}

func (t *Task) MakeRunable(newUUID string, runCount int) *Task {
	emptyStr := ""
	newTask := &Task{
		UUID:     newUUID,
		Status:   Available,
		WorkedBy: &emptyStr,
		Stage:    "",
		RunCount: int64(runCount),
	}

	if rtm := t.RetrievalTask; rtm != nil {
		newTask.RetrievalTask = &RetrievalTask{
			Miner:           rtm.Miner,
			PayloadCID:      rtm.PayloadCID,
			CARExport:       rtm.CARExport,
			Tag:             rtm.Tag,
			MaxPriceAttoFIL: rtm.MaxPriceAttoFIL,
		}
	}
	if stm := t.StorageTask; stm != nil {
		newTask.StorageTask = &StorageTask{
			Miner:           stm.Miner,
			MaxPriceAttoFIL: stm.MaxPriceAttoFIL,
			Size:            stm.Size,
			StartOffset:     stm.StartOffset,
			FastRetrieval:   stm.FastRetrieval,
			Verified:        stm.Verified,
			Tag:             stm.Tag,
		}
	}
	return newTask
}

func (t *Task) HasSchedule() bool {
	if rt := t.RetrievalTask; rt != nil {
		if sch := rt.Schedule; sch != nil {
			return *sch != ""
		}
	} else if st := t.StorageTask; st != nil {
		if sch := st.Schedule; sch != nil {
			return *sch != ""
		}
	}
	return false
}

func (t *Task) Schedule() (string, string) {
	var schedule, limit string

	if rt := t.RetrievalTask; rt != nil {
		if sch := rt.Schedule; sch != nil {
			schedule = *sch
			if lim := rt.ScheduleLimit; lim != nil {
				limit = *lim
			}
		}
	} else if st := t.StorageTask; st != nil {
		if sch := st.Schedule; sch != nil {
			schedule = *sch
			if lim := st.ScheduleLimit; lim != nil {
				limit = *lim
			}
		}
	}
	return schedule, limit
}

func (t *Task) Tag() string {
	if rt := t.RetrievalTask; rt != nil {
		if tag := rt.Tag; tag != nil {
			return *tag
		}
	}
	if st := t.StorageTask; st != nil {
		if tag := st.Tag; tag != nil {
			return *tag
		}
	}
	return ""
}

func (t *Task) Assign(worker string, status Status) *Task {
	timenow := Time(time.Now().UnixNano())
	newTask := &Task{
		UUID:                t.UUID,
		Status:              status,
		WorkedBy:            &worker,
		Stage:               t.Stage,
		CurrentStageDetails: t.CurrentStageDetails,
		PastStageDetails:    t.PastStageDetails,
		StartedAt:           &timenow,
		RetrievalTask:       t.RetrievalTask,
		RunCount:            t.RunCount,
		StorageTask:         t.StorageTask,
	}

	//todo: sign
	return newTask
}

func (t *Task) Finalize(ctx context.Context, ls ipld.LinkSystem, local bool) (*FinishedTask, error) {
	if !local {
		if t.Status != Failed && t.Status != Successful {
			return nil, fmt.Errorf("task cannot be finalized as it is not in a finished state")
		}
	}

	logs := parseFinalLogs(t)
	ft := FinishedTask{
		Status:             t.Status,
		StartedAt:          *(t.StartedAt),
		ErrorMessage:       t.ErrorMessage,
		RetrievalTask:      t.RetrievalTask,
		StorageTask:        t.StorageTask,
		DealID:             int64(logs.dealID),
		MinerMultiAddr:     logs.minerAddr,
		ClientApparentAddr: logs.clientAddr,
		MinerLatencyMS:     logs.minerLatency,
		TimeToFirstByteMS:  logs.timeFirstByte,
		TimeToLastByteMS:   logs.timeLastByte,
		MinerVersion:       logs.minerVersion,
		ClientVersion:      logs.clientVersion,
		Size:               logs.size,
		PayloadCID:         logs.payloadCID,
		ProposalCID:        logs.proposalCID,
		DealIDString:       logs.DealIDString(),
		MinerPeerID:        logs.minerPeerID,
	}
	// events to dag item
	logList := StageDetailsList{}
	if len(t.PastStageDetails) != 0 {
		logList = t.PastStageDetails
	}
	n, err := logList.ToNode()
	if err != nil {
		return nil, fmt.Errorf("failed to convert loglist to ipld node, err: %v", err)
	}

	logLnk, err := ls.Store(ipld.LinkContext{}, linkProto, n)
	if err != nil {
		return nil, err
	}
	ft.Events = logLnk

	return &ft, nil
}

type logExtraction struct {
	dealID        int
	minerAddr     string
	minerVersion  *string
	clientAddr    string
	clientVersion *string
	minerLatency  *int64
	timeFirstByte *int64
	timeLastByte  *int64
	size          *int64
	payloadCID    *string
	proposalCID   *string
	minerPeerID   *string
}

func (l *logExtraction) DealIDString() *string {
	if l.dealID == 0 {
		return nil
	}
	str := strconv.Itoa(l.dealID)
	return &str
}
func parseFinalLogs(t *Task) *logExtraction {
	le := new(logExtraction)

	if t.StorageTask != nil {
		// copy value instead of ptr
		sz := t.StorageTask.Size
		le.size = &sz
	} else if t.RetrievalTask != nil {
		pcid := t.RetrievalTask.PayloadCID
		le.payloadCID = &pcid
	}

	// If the task failed early, we might not have some of the info.
	if t.StartedAt == nil || t.PastStageDetails == nil {
		return le
	}

	// First/last byte stage substrings for tasks.
	// They get matched

	// If the provider is verifying the data, it has all the bytes.
	firstByteSubstring := "opening data transfer"
	lastByteSubstring := "provider is verifying the data"

	// First/last byte log entries for retrieval tasks.
	// From RetrievalStages above; matches stage descriptions.
	if t.RetrievalTask == nil {
		firstByteSubstring = "First byte of data received"
		lastByteSubstring = "All bytes received"
	}

	start := t.StartedAt.Time()
	for _, stage := range t.PastStageDetails {

		// First/last byte deriving also looks at stage descriptions.
		if stage.UpdatedAt != nil && stage.Description != nil {
			entry := *(stage.Description)
			entryTime := stage.UpdatedAt.Time()
			t := entryTime.Sub(start).Milliseconds()
			if le.timeFirstByte == nil && strings.Contains(entry, firstByteSubstring) {
				le.timeFirstByte = &t
			} else if le.timeLastByte == nil && strings.Contains(entry, lastByteSubstring) {
				le.timeLastByte = &t
			}
		}

		for _, log := range stage.Logs {
			entry := log.Log
			entryTime := log.UpdatedAt.Time()

			if le.timeFirstByte == nil && strings.Contains(entry, firstByteSubstring) {
				t := entryTime.Sub(start).Milliseconds()
				le.timeFirstByte = &t
			} else if le.timeLastByte == nil && strings.Contains(entry, lastByteSubstring) {
				t := entryTime.Sub(start).Milliseconds()
				le.timeLastByte = &t
			}

			if le.minerVersion == nil && strings.Contains(entry, "NetAgentVersion: ") {
				str := strings.TrimPrefix(entry, "NetAgentVersion: ")
				le.minerVersion = &str
			}
			if le.clientVersion == nil && strings.Contains(entry, "ClientVersion: ") {
				str := strings.TrimPrefix(entry, "ClientVersion: ")
				le.clientVersion = &str
			}
			if le.minerAddr == "" && strings.Contains(entry, "RemotePeerAddr: ") {
				le.minerAddr = strings.TrimPrefix(entry, "RemotePeerAddr: ")
			}
			if le.minerLatency == nil && strings.Contains(entry, "RemotePeerLatency: ") {
				val, err := strconv.Atoi(strings.TrimPrefix(entry, "RemotePeerLatency: "))
				if err == nil {
					minerLatency := int64(val)
					le.minerLatency = &minerLatency
				}
			}
			if le.size == nil && strings.Contains(entry, "bytes received:") {
				b, err := strconv.ParseInt(strings.TrimPrefix(entry, "bytes received: "), 10, 10)
				if err == nil {
					le.size = &b
				}
			}
			if le.payloadCID == nil && strings.Contains(entry, "PayloadCID:") {
				str := strings.TrimPrefix(entry, "PayloadCID: ")
				le.payloadCID = &str
			}
			if le.proposalCID == nil && strings.Contains(entry, "ProposalCID:") {
				str := strings.TrimPrefix(entry, "ProposalCID: ")
				le.proposalCID = &str
			}
			if le.dealID == 0 && strings.Contains(entry, "DealID:") {
				val, err := strconv.Atoi(strings.TrimPrefix(entry, "DealID: "))
				if err == nil {
					le.dealID = val
				}
			}
			if le.minerPeerID == nil && strings.Contains(entry, "RemotePeerID:") {
				str := strings.TrimPrefix(entry, "RemotePeerID: ")
				le.minerPeerID = &str
			}
		}
	}

	// Sometimes we'll see an event for the first byte and no event for the
	// last byte, such as when doing a retrieval task and the data is
	// already present locally.
	// In those casese, assume that the transfer was immediate.
	if le.timeFirstByte != nil && le.timeLastByte == nil {
		le.timeLastByte = le.timeFirstByte
	}

	return le
}

func (t *Task) UpdateTask(tsk *UpdateTask) (*Task, error) {
	stage := ""
	if tsk.Stage != nil {
		stage = *(tsk.Stage)
	}
	// don use &tsk.WorkBy, ptr may change
	workby := tsk.WorkedBy
	updatedTask := &Task{
		UUID:                t.UUID,
		Status:              tsk.Status,
		WorkedBy:            &workby,
		ErrorMessage:        tsk.ErrorMessage,
		Stage:               stage,
		StartedAt:           t.StartedAt,
		RunCount:            tsk.RunCount,
		CurrentStageDetails: tsk.CurrentStageDetails,
		RetrievalTask:       t.RetrievalTask,
		StorageTask:         t.StorageTask,
	}

	// On stage transitions, archive the current stage.
	if stage != t.Stage && t.CurrentStageDetails != nil {
		if t.PastStageDetails == nil {
			updatedTask.PastStageDetails = StageDetailsList{*t.CurrentStageDetails}
		} else {
			updatedTask.PastStageDetails = append(t.PastStageDetails, *t.CurrentStageDetails)
		}
	} else {
		updatedTask.PastStageDetails = t.PastStageDetails
	}

	return updatedTask, nil
}

func (t *Task) GetUUID() string {
	return t.UUID
}

//func (tl *TasksPrototype) Of(ts []Task) *_Tasks {
//	t := _Tasks{
//		x: []_Task{},
//	}
//	for _, c := range ts {
//		t.x = append(t.x, *c)
//	}
//	return &t
//}
//
func (ts *Tasks) List() []*Task {
	itmsp := make([]Task, len(*ts))
	itms := make([]*Task, len(*ts))
	for i := range *ts {
		itmsp[i] = (*ts)[i]
		itms[i] = &itmsp[i]
	}
	return itms
}

//func (arp *_AuthenticatedRecord__Prototype) Of(c cid.Cid, sig []byte) *_AuthenticatedRecord {
//	ar := _AuthenticatedRecord{
//		Record:    _Link_FinishedTask{x: linksystem.Link{Cid: c}},
//		Signature: _Bytes{x: sig},
//	}
//	return &ar
//}

func NewAuthenticatedRecord(c cid.Cid, sig []byte) *AuthenticatedRecord {
	ar := AuthenticatedRecord{
		Record:    linksystem.Link{Cid: c},
		Signature: sig,
	}
	return &ar
}

func NewAuthenticatedRecordList(ars []*AuthenticatedRecord) *[]AuthenticatedRecord {
	al := make([]AuthenticatedRecord, 0)
	for _, a := range ars {
		al = append(al, *a)
	}
	return &al
}

//func (alp *_List_AuthenticatedRecord__Prototype) Of(ars []*_AuthenticatedRecord) *_List_AuthenticatedRecord {
//	al := _List_AuthenticatedRecord{
//		x: []_AuthenticatedRecord{},
//	}
//	for _, a := range ars {
//		al.x = append(al.x, *a)
//	}
//	return &al
//}
//

func NewRecordUpdate(rcrds *[]AuthenticatedRecord, previous cid.Cid, previousSig []byte) *RecordUpdate {
	var lrm *ipld.Link
	if previous != cid.Undef {
		lk := ipld.Link(linksystem.Link{Cid: previous})
		lrm = &lk
	}
	ru := RecordUpdate{
		Records:  *rcrds,
		SigPrev:  previousSig,
		Previous: lrm,
	}
	return &ru
}

//func (rup *_RecordUpdate__Prototype) Of(rcrds *_List_AuthenticatedRecord, previous cid.Cid, previousSig []byte) *_RecordUpdate {
//	lrm := _Link__Maybe{m: schema.Maybe_Null, v: _Link{}}
//	if previous != cid.Undef {
//		lrm.m = schema.Maybe_Value
//		lrm.v = _Link{x: linksystem.Link{Cid: previous}}
//	}
//	ru := _RecordUpdate{
//		Records:  *rcrds,
//		SigPrev:  _Bytes{x: previousSig},
//		Previous: lrm,
//	}
//	return &ru
//}
//
//func (tl *_FinishedTasks__Prototype) Of(ts []FinishedTask) *_FinishedTasks {
//	t := _FinishedTasks{
//		x: make([]_FinishedTask, 0, len(ts)),
//	}
//	for _, c := range ts {
//		t.x = append(t.x, *c)
//	}
//	return &t
//}
