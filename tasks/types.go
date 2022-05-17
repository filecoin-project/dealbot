package tasks

import (
	"errors"
	"fmt"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
)

type PopTask struct {
	Status   int64
	WorkedBy string
	Tag      []string
}

type RetrievalTask struct {
	Miner           string
	PayloadCID      string
	CARExport       bool
	Schedule        string
	ScheduleLimit   string
	Tag             string
	MaxPriceAttoFIL int
}

type AuthenticatedRecord struct {
	Record    ipld.Link
	Signature []byte
}

type RecordUpdate struct {
	Records  []*AuthenticatedRecord
	SigPrev  []byte
	Previous *ipld.Link
}

type StorageTask struct {
	Miner                    string
	MaxPriceAttoFIL          int
	Size                     int
	StartOffset              int
	FastRetrieval            bool
	Verified                 bool
	Schedule                 string
	ScheduleLimit            string
	Tag                      string
	RetrievalSchedule        string
	RetrievalScheduleLimit   string
	RetrievalMaxPriceAttoFIL int
}

type Logs struct {
	Log       string
	UpdatedAt int64
}

type StageDetails struct {
	Description      string
	ExpectedDuration string
	Logs             []*Logs
	UpdatedAt        int64
}

type UpdateTask struct {
	Status              int64
	ErrorMessage        string
	Stage               string
	CurrentStageDetails *StageDetails
	WorkedBy            string
	RunCount            int
}

type StageDetailsList []*StageDetails

type Task struct {
	UUID                string
	Status              int64
	WorkedBy            string
	Stage               string
	CurrentStageDetails *StageDetails
	PastStageDetails    StageDetailsList
	Started             int64
	RunCount            int64
	ErrorMessage        string
	RetrievalTask       *RetrievalTask
	StorageTask         *StorageTask
}

type FinishedTask struct {
	Status             int64
	StartedAt          int64
	ErrorMessage       string
	RetrievalTask      *RetrievalTask
	StorageTask        *StorageTask
	DealID             int64
	MinerMultiAddr     string
	ClientApparentAddr string
	MinerLatencyMS     int
	TimeToFirstByteMS  int
	TimeToLastByteMS   int
	Events             ipld.Link
	MinerVersion       string
	ClientVersion      string
	Size               int
	PayloadCID         string
	ProposalCID        string
	DealIDString       string
	MinerPeerID        string
}

func (t *Task) ToNode() (n ipld.Node, err error) {
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
		adBuilder := TaskPrototype.NewBuilder()
		err := adBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = adBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*Task)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (f *FinishedTask) ToNode() (n ipld.Node, err error) {
	// TODO: remove the panic recovery once IPLD bindnode is stabilized.
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
		adBuilder := FinishedTaskPrototype.NewBuilder()
		err := adBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = adBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*FinishedTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (s *StorageTask) ToNode() (n ipld.Node, err error) {
	// TODO: remove the panic recovery once IPLD bindnode is stabilized.
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
		adBuilder := StorageTaskPrototype.NewBuilder()
		err := adBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = adBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*StorageTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (u *UpdateTask) ToNode() (n ipld.Node, err error) {
	// TODO: remove the panic recovery once IPLD bindnode is stabilized.
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
		adBuilder := UpdatedTaskPrototype.NewBuilder()
		err := adBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = adBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*UpdateTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
}

func (r *RetrievalTask) ToNode() (n ipld.Node, err error) {
	// TODO: remove the panic recovery once IPLD bindnode is stabilized.
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
		adBuilder := RetrievalTaskPrototype.NewBuilder()
		err := adBuilder.AssignNode(node)
		if err != nil {
			return nil, fmt.Errorf("faild to convert node prototype: %w", err)
		}
		node = adBuilder.Build()
	}

	t, ok := bindnode.Unwrap(node).(*RetrievalTask)
	if !ok || t == nil {
		return nil, fmt.Errorf("unwrapped node does not match schema.Task")
	}
	return t, nil
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
