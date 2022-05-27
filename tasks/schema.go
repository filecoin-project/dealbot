package tasks

import (
	_ "embed"
	"fmt"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
)

var (
	TaskPrototype                schema.TypedPrototype
	TasksPrototype               schema.TypedPrototype
	FinishedTaskPrototype        schema.TypedPrototype
	RetrievalTaskPrototype       schema.TypedPrototype
	LogsPrototype                schema.TypedPrototype
	AuthenticatedRecordPrototype schema.TypedPrototype
	StageDetailsPrototype        schema.TypedPrototype
	StageDetailsListPrototype    schema.TypedPrototype
	CurrentStageDetailsPrototype schema.TypedPrototype
	StorageTaskPrototype         schema.TypedPrototype
	PopTaskPrototype             schema.TypedPrototype
	UpdatedTaskPrototype         schema.TypedPrototype
	RecordUpdatePrototype        schema.TypedPrototype
	//go:embed schema.ipldsch
	schemaBytes []byte
)

func init() {
	typeSystem, err := ipld.LoadSchemaBytes(schemaBytes)
	if err != nil {
		panic(fmt.Errorf("failed to load schema: %w", err))
	}
	LogsPrototype = bindnode.Prototype((*Logs)(nil), typeSystem.TypeByName("Logs"))
	//CurrentStageDetailsPrototype = bindnode.Prototype((*))x
	StageDetailsPrototype = bindnode.Prototype((*StageDetails)(nil), typeSystem.TypeByName("StageDetails"))
	StageDetailsListPrototype = bindnode.Prototype((*StageDetailsList)(nil), typeSystem.TypeByName("StageDetailsList"))
	StorageTaskPrototype = bindnode.Prototype((*StorageTask)(nil), typeSystem.TypeByName("StorageTask"))
	UpdatedTaskPrototype = bindnode.Prototype((*UpdateTask)(nil), typeSystem.TypeByName("UpdateTask"))
	RecordUpdatePrototype = bindnode.Prototype((*RecordUpdate)(nil), typeSystem.TypeByName("RecordUpdate"))
	PopTaskPrototype = bindnode.Prototype((*PopTask)(nil), typeSystem.TypeByName("PopTask"))
	AuthenticatedRecordPrototype = bindnode.Prototype((*AuthenticatedRecord)(nil), typeSystem.TypeByName("AuthenticatedRecord"))
	TaskPrototype = bindnode.Prototype((*Task)(nil), typeSystem.TypeByName("Task"))
	TasksPrototype = bindnode.Prototype((*Tasks)(nil), typeSystem.TypeByName("Tasks"))
	FinishedTaskPrototype = bindnode.Prototype((*FinishedTask)(nil), typeSystem.TypeByName("FinishedTask"))
	RetrievalTaskPrototype = bindnode.Prototype((*RetrievalTask)(nil), typeSystem.TypeByName("RetrievalTask"))

}
