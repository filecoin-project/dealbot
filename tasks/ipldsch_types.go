package tasks

// Code generated by go-ipld-prime gengo.  DO NOT EDIT.

import (
	ipld "github.com/ipld/go-ipld-prime"
)
var _ ipld.Node = nil // suppress errors when this dependency is not referenced
// Type is a struct embeding a NodePrototype/Type for every Node implementation in this package.
// One of its major uses is to start the construction of a value.
// You can use it like this:
//
// 		tasks.Type.YourTypeName.NewBuilder().BeginMap() //...
//
// and:
//
// 		tasks.Type.OtherTypeName.NewBuilder().AssignString("x") // ...
//
var Type typeSlab

type typeSlab struct {
	Any       _Any__Prototype
	Any__Repr _Any__ReprPrototype
	Bool       _Bool__Prototype
	Bool__Repr _Bool__ReprPrototype
	Bytes       _Bytes__Prototype
	Bytes__Repr _Bytes__ReprPrototype
	Float       _Float__Prototype
	Float__Repr _Float__ReprPrototype
	Int       _Int__Prototype
	Int__Repr _Int__ReprPrototype
	Link       _Link__Prototype
	Link__Repr _Link__ReprPrototype
	List       _List__Prototype
	List__Repr _List__ReprPrototype
	List_Logs       _List_Logs__Prototype
	List_Logs__Repr _List_Logs__ReprPrototype
	Logs       _Logs__Prototype
	Logs__Repr _Logs__ReprPrototype
	Map       _Map__Prototype
	Map__Repr _Map__ReprPrototype
	PopTask       _PopTask__Prototype
	PopTask__Repr _PopTask__ReprPrototype
	RetrievalTask       _RetrievalTask__Prototype
	RetrievalTask__Repr _RetrievalTask__ReprPrototype
	StageDetails       _StageDetails__Prototype
	StageDetails__Repr _StageDetails__ReprPrototype
	Status       _Status__Prototype
	Status__Repr _Status__ReprPrototype
	StorageTask       _StorageTask__Prototype
	StorageTask__Repr _StorageTask__ReprPrototype
	String       _String__Prototype
	String__Repr _String__ReprPrototype
	Task       _Task__Prototype
	Task__Repr _Task__ReprPrototype
	Tasks       _Tasks__Prototype
	Tasks__Repr _Tasks__ReprPrototype
	Time       _Time__Prototype
	Time__Repr _Time__ReprPrototype
	UpdateTask       _UpdateTask__Prototype
	UpdateTask__Repr _UpdateTask__ReprPrototype
}

// --- type definitions follow ---

// Any matches the IPLD Schema type "Any".  It has Union type-kind, and may be interrogated like map kind.
type Any = *_Any
type _Any struct {
	x _Any__iface
}
type _Any__iface interface {
	_Any__member()
}
func (_Bool) _Any__member() {}
func (_Int) _Any__member() {}
func (_Float) _Any__member() {}
func (_String) _Any__member() {}
func (_Bytes) _Any__member() {}
func (_Map) _Any__member() {}
func (_List) _Any__member() {}
func (_Link) _Any__member() {}

// Bool matches the IPLD Schema type "Bool".  It has bool kind.
type Bool = *_Bool
type _Bool struct{ x bool }

// Bytes matches the IPLD Schema type "Bytes".  It has bytes kind.
type Bytes = *_Bytes
type _Bytes struct{ x []byte }

// Float matches the IPLD Schema type "Float".  It has float kind.
type Float = *_Float
type _Float struct{ x float64 }

// Int matches the IPLD Schema type "Int".  It has int kind.
type Int = *_Int
type _Int struct{ x int64 }

// Link matches the IPLD Schema type "Link".  It has link kind.
type Link = *_Link
type _Link struct{ x ipld.Link }

// List matches the IPLD Schema type "List".  It has list kind.
type List = *_List
type _List struct {
	x []_Any__Maybe
}

// List_Logs matches the IPLD Schema type "List_Logs".  It has list kind.
type List_Logs = *_List_Logs
type _List_Logs struct {
	x []_Logs
}

// Logs matches the IPLD Schema type "Logs".  It has Struct type-kind, and may be interrogated like map kind.
type Logs = *_Logs
type _Logs struct {
	Log _String
	UpdatedAt _Time
}

// Map matches the IPLD Schema type "Map".  It has map kind.
type Map = *_Map
type _Map struct {
	m map[_String]MaybeAny
	t []_Map__entry
}
type _Map__entry struct {
	k _String
	v _Any__Maybe
}

// PopTask matches the IPLD Schema type "PopTask".  It has Struct type-kind, and may be interrogated like map kind.
type PopTask = *_PopTask
type _PopTask struct {
	Status _Status
	WorkedBy _String
}

// RetrievalTask matches the IPLD Schema type "RetrievalTask".  It has Struct type-kind, and may be interrogated like map kind.
type RetrievalTask = *_RetrievalTask
type _RetrievalTask struct {
	Miner _String
	PayloadCID _String
	CARExport _Bool
}

// StageDetails matches the IPLD Schema type "StageDetails".  It has Struct type-kind, and may be interrogated like map kind.
type StageDetails = *_StageDetails
type _StageDetails struct {
	Description _String__Maybe
	ExpectedDuration _String__Maybe
	Logs _List_Logs
	UpdatedAt _Time__Maybe
}

// Status matches the IPLD Schema type "Status".  It has int kind.
type Status = *_Status
type _Status struct{ x int64 }

// StorageTask matches the IPLD Schema type "StorageTask".  It has Struct type-kind, and may be interrogated like map kind.
type StorageTask = *_StorageTask
type _StorageTask struct {
	Miner _String
	MaxPriceAttoFIL _Int
	Size _Int
	StartOffset _Int
	FastRetrieval _Bool
	Verified _Bool
}

// String matches the IPLD Schema type "String".  It has string kind.
type String = *_String
type _String struct{ x string }

// Task matches the IPLD Schema type "Task".  It has Struct type-kind, and may be interrogated like map kind.
type Task = *_Task
type _Task struct {
	UUID _String
	Status _Status
	WorkedBy _String__Maybe
	Stage _String
	CurrentStageDetails _StageDetails__Maybe
	StartedAt _Time__Maybe
	RetrievalTask _RetrievalTask__Maybe
	StorageTask _StorageTask__Maybe
}

// Tasks matches the IPLD Schema type "Tasks".  It has list kind.
type Tasks = *_Tasks
type _Tasks struct {
	x []_Task
}

// Time matches the IPLD Schema type "Time".  It has int kind.
type Time = *_Time
type _Time struct{ x int64 }

// UpdateTask matches the IPLD Schema type "UpdateTask".  It has Struct type-kind, and may be interrogated like map kind.
type UpdateTask = *_UpdateTask
type _UpdateTask struct {
	Status _Status
	Stage _String__Maybe
	CurrentStageDetails _StageDetails__Maybe
	WorkedBy _String
}

