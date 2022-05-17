package tasks

import (
	_ "embed"
	"fmt"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
)

var (
	// Linkproto is the ipld.LinkProtocol used for the legs protocol.
	// Refer to it if you have encoding questions.
	//LinkProto = cidlink.LinkPrototype{
	//	Prefix: cid.Prefix{
	//		Version:  1,
	//		Codec:    uint64(multicodec.DagJson),
	//		MhType:   uint64(multicodec.Sha2_256),
	//		MhLength: 16,
	//	},
	//}

	// MetadataPrototype represents the IPLD node prototype of Metadata.
	// See: bindnode.Prototype.
	TaskPrototype                schema.TypedPrototype
	FinishedTaskPrototype        schema.TypedPrototype
	RetrievalTaskPrototype       schema.TypedPrototype
	LogsPrototype                schema.TypedPrototype
	AuthenticatedRecordPrototype schema.TypedPrototype
	StageDetailsPrototype        schema.TypedPrototype
	CurrentStageDetailsPrototype schema.TypedPrototype
	StorageTaskPrototype         schema.TypedPrototype
	UpdatedTaskPrototype         schema.TypedPrototype
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
	StorageTaskPrototype = bindnode.Prototype((*StorageTask)(nil), typeSystem.TypeByName("StorageTask"))
	UpdatedTaskPrototype = bindnode.Prototype((*UpdateTask)(nil), typeSystem.TypeByName("UpdateTask"))
	AuthenticatedRecordPrototype = bindnode.Prototype((*AuthenticatedRecord)(nil), typeSystem.TypeByName("AuthenticatedRecord"))
	TaskPrototype = bindnode.Prototype((*Task)(nil), typeSystem.TypeByName("Task"))
	FinishedTaskPrototype = bindnode.Prototype((*FinishedTask)(nil), typeSystem.TypeByName("FinishedTask"))
	RetrievalTaskPrototype = bindnode.Prototype((*RetrievalTask)(nil), typeSystem.TypeByName("RetrievalTask"))

}
