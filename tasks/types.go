package tasks

import (
	"errors"
	"fmt"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
)

type Status int64
type Time int64

type PopTask struct {
	Status   Status
	WorkedBy string
	Tags     []string
}

type RetrievalTask struct {
	Miner           string
	PayloadCID      string
	CARExport       bool
	Schedule        *string
	ScheduleLimit   *string
	Tag             *string
	MaxPriceAttoFIL *int64 // nil Exist() ???   &(int64(0))
}

type AuthenticatedRecord struct {
	Record    ipld.Link
	Signature []byte
}

type AuthenticatedRecordList []AuthenticatedRecord

type RecordUpdate struct {
	Records  AuthenticatedRecordList
	SigPrev  []byte
	Previous *ipld.Link
}

type StorageTask struct {
	Miner                    string
	MaxPriceAttoFIL          int64
	Size                     int64
	StartOffset              int64
	FastRetrieval            bool
	Verified                 bool
	Schedule                 *string
	ScheduleLimit            *string
	Tag                      *string
	RetrievalSchedule        *string
	RetrievalScheduleLimit   *string
	RetrievalMaxPriceAttoFIL *int64
}

type Logs struct {
	Log       string
	UpdatedAt Time
}

type StageDetails struct {
	Description      *string
	ExpectedDuration *string
	Logs             []Logs
	UpdatedAt        *Time
}

type UpdateTask struct {
	Status              Status
	ErrorMessage        *string
	Stage               *string
	CurrentStageDetails *StageDetails
	WorkedBy            string
	RunCount            int64
}

type StageDetailsList []StageDetails

type Task struct {
	UUID                string
	Status              Status
	WorkedBy            *string
	Stage               string
	CurrentStageDetails *StageDetails
	PastStageDetails    StageDetailsList
	StartedAt           *Time
	RunCount            int64
	ErrorMessage        *string
	RetrievalTask       *RetrievalTask
	StorageTask         *StorageTask
}

type Tasks []Task

type FinishedTask struct {
	Status             Status
	StartedAt          Time
	ErrorMessage       *string
	RetrievalTask      *RetrievalTask
	StorageTask        *StorageTask
	DealID             int64
	MinerMultiAddr     string
	ClientApparentAddr string
	MinerLatencyMS     *int64
	TimeToFirstByteMS  *int64
	TimeToLastByteMS   *int64
	Events             ipld.Link
	MinerVersion       *string
	ClientVersion      *string
	Size               *int64
	PayloadCID         *string
	ProposalCID        *string
	DealIDString       *string
	MinerPeerID        *string
}

type FinishedTasks []FinishedTask

func (t Task) ToNode() (n ipld.Node, err error) {
	// TODO: remove the panic recovery once IPLD bindnode is stabilized.
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&t, TaskPrototype.Type()).Representation()
	return
}

// UnwrapTask unwraps the given node as a Task.
//
// Note that the node is reassigned to TaskPrototype if its prototype is different.
// Therefore, it is recommended to load the node using the correct prototype initially
// function to avoid unnecessary node assignment.
func UnwrapTask(node ipld.Node) (*Task, error) {
	// When an IPLD node is loaded using `Prototype.Any` unwrap with bindnode will not work.
	// Here we defensively check the prototype and wrap if needed, since:
	//   - linksystem in sti is passed into other libraries, like go-legs, and
	//   - for whatever reason clients of this package may load nodes using Prototype.Any.
	//
	// The code in this repo, however should load nodes with appropriate prototype and never trigger
	// this if statement.
	if node.Prototype() != TaskPrototype {
		tsBuilder := TaskPrototype.NewBuilder()
		err := tsBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = tsBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*Task)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (f FinishedTask) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&f, FinishedTaskPrototype.Type()).Representation()
	return
}

func UnwrapFinishedTask(node ipld.Node) (*FinishedTask, error) {
	if node.Prototype() != FinishedTaskPrototype {
		tsBuilder := FinishedTaskPrototype.NewBuilder()
		err := tsBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = tsBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*FinishedTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (s StorageTask) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&s, StorageTaskPrototype.Type()).Representation()
	return
}

func UnwrapStorageTask(node ipld.Node) (*StorageTask, error) {
	if node.Prototype() != StorageTaskPrototype {
		tsBuilder := StorageTaskPrototype.NewBuilder()
		err := tsBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = tsBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*StorageTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (u UpdateTask) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&u, UpdatedTaskPrototype.Type()).Representation()
	return
}

func UnwrapUpdateTask(node ipld.Node) (*UpdateTask, error) {
	if node.Prototype() != UpdatedTaskPrototype {
		tsBuilder := UpdatedTaskPrototype.NewBuilder()
		err := tsBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = tsBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*UpdateTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (r RetrievalTask) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&r, RetrievalTaskPrototype.Type()).Representation()
	return
}

func UnwrapRetrievalTask(node ipld.Node) (*RetrievalTask, error) {
	if node.Prototype() != RetrievalTaskPrototype {
		tsBuilder := RetrievalTaskPrototype.NewBuilder()
		err := tsBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = tsBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*RetrievalTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func NewTasks(ts []*Task) *Tasks {
	t := Tasks{}
	for _, c := range ts {
		t = append(t, *c)
	}
	return &t
}

func (ts Tasks) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&ts, TasksPrototype.Type()).Representation()
	return
}

func UnwrapTasks(node ipld.Node) (*Tasks, error) {
	if node.Prototype() != TasksPrototype {
		tsBuilder := TasksPrototype.NewBuilder()
		err := tsBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = tsBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*Tasks)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (pt PopTask) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&pt, PopTaskPrototype.Type()).Representation()
	return
}

func UnwrapPopTask(node ipld.Node) (*PopTask, error) {
	if node.Prototype() != PopTaskPrototype {
		ptBuilder := PopTaskPrototype.NewBuilder()
		err := ptBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = ptBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*PopTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (sdl StageDetailsList) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&sdl, StageDetailsListPrototype.Type()).Representation()
	return
}

func UnwrapStageDetailsList(node ipld.Node) (*StageDetailsList, error) {
	if node.Prototype() != StageDetailsListPrototype {
		sdlBuilder := StageDetailsListPrototype.NewBuilder()
		err := sdlBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = sdlBuilder.Build()
	}

	s, ok := bindnode.Unwrap(node).(*StageDetailsList)
	if !ok || s == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return s, nil
}

func (sd StageDetails) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&sd, StageDetailsPrototype.Type()).Representation()
	return
}

func UnwrapStageDetails(node ipld.Node) (*StageDetails, error) {
	if node.Prototype() != StageDetailsPrototype {
		sdBuilder := StageDetailsPrototype.NewBuilder()
		err := sdBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = sdBuilder.Build()
	}

	s, ok := bindnode.Unwrap(node).(*StageDetails)
	if !ok || s == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return s, nil
}

func (l Logs) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&l, LogsPrototype.Type()).Representation()
	return
}

func UnwrapLogs(node ipld.Node) (*Logs, error) {
	if node.Prototype() != LogsPrototype {
		lBuilder := LogsPrototype.NewBuilder()
		err := lBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = lBuilder.Build()
	}

	l, ok := bindnode.Unwrap(node).(*Logs)
	if !ok || l == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return l, nil
}

func (ru RecordUpdate) ToNode() (n ipld.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = toError(r)
		}
	}()
	n = bindnode.Wrap(&ru, RecordUpdatePrototype.Type()).Representation()
	return
}

func UnwrapRecordUpdate(node ipld.Node) (*RecordUpdate, error) {
	if node.Prototype() != RecordUpdatePrototype {
		ruBuilder := RecordUpdatePrototype.NewBuilder()
		err := ruBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = ruBuilder.Build()
	}

	ru, ok := bindnode.Unwrap(node).(*RecordUpdate)
	if !ok || ru == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return ru, nil
}

func toError(r interface{}) error {
	switch x := r.(type) {
	case string:
		return errors.New(x)
	case error:
		return x
	default:
		return fmt.Errorf("unknown panic: %v", r)
	}
}
