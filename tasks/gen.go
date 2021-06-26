// +build ignore

package main

import (
	"fmt"
	"os"
	"path"

	gengraphql "github.com/ipld/go-ipld-graphql/gen"
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/schema"
	gengo "github.com/ipld/go-ipld-prime/schema/gen/go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Must specify destination directory")
		os.Exit(1)
	}

	ts := schema.TypeSystem{}
	ts.Init()
	adjCfg := &gengo.AdjunctCfg{
		CfgUnionMemlayout: map[schema.TypeName]string{
			"Any": "interface",
		},
	}

	// Prelude.  (This is boilerplate; it will be injected automatically by the schema libraries in the future, but isn't yet.)
	ts.Accumulate(schema.SpawnBool("Bool"))
	ts.Accumulate(schema.SpawnInt("Int"))
	ts.Accumulate(schema.SpawnFloat("Float"))
	ts.Accumulate(schema.SpawnString("String"))
	ts.Accumulate(schema.SpawnBytes("Bytes"))
	ts.Accumulate(schema.SpawnMap("Map",
		"String", "Any", true,
	))
	ts.Accumulate(schema.SpawnList("List",
		"Any", true,
	))
	ts.Accumulate(schema.SpawnLink("Link"))
	ts.Accumulate(schema.SpawnUnion("Any",
		[]schema.TypeName{
			"Bool",
			"Int",
			"Float",
			"String",
			"Bytes",
			"Map",
			"List",
			"Link",
		},

		schema.SpawnUnionRepresentationKinded(map[ipld.Kind]schema.TypeName{
			ipld.Kind_Bool:   "Bool",
			ipld.Kind_Int:    "Int",
			ipld.Kind_Float:  "Float",
			ipld.Kind_String: "String",
			ipld.Kind_Bytes:  "Bytes",
			ipld.Kind_Map:    "Map",
			ipld.Kind_List:   "List",
			ipld.Kind_Link:   "Link",
		}),
	))

	// Our types.
	ts.Accumulate(schema.SpawnInt("Time"))

	ts.Accumulate(schema.SpawnInt("Status"))

	ts.Accumulate(schema.SpawnList("List_String", "String", false))

	ts.Accumulate(schema.SpawnStruct("StageDetails", []schema.StructField{
		schema.SpawnStructField("Description", "String", true, false),
		schema.SpawnStructField("ExpectedDuration", "String", true, false),
		schema.SpawnStructField("Logs", "List_Logs", false, false),
		schema.SpawnStructField("UpdatedAt", "Time", true, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))
	ts.Accumulate(schema.SpawnList("List_StageDetails", "StageDetails", false))
	ts.Accumulate(schema.SpawnLinkReference("Link_List_StageDetails", "List_StageDetails"))

	ts.Accumulate(schema.SpawnList("List_Logs", "Logs", false))
	ts.Accumulate(schema.SpawnStruct("Logs", []schema.StructField{
		schema.SpawnStructField("Log", "String", false, false),
		schema.SpawnStructField("UpdatedAt", "Time", false, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))

	ts.Accumulate(schema.SpawnStruct("RetrievalTask", []schema.StructField{
		schema.SpawnStructField("Miner", "String", false, false),
		schema.SpawnStructField("PayloadCID", "String", false, false),
		schema.SpawnStructField("CARExport", "Bool", false, false),
		schema.SpawnStructField("Schedule", "String", true, true),
		schema.SpawnStructField("ScheduleLimit", "String", true, true),
		schema.SpawnStructField("Tag", "String", true, true),
		schema.SpawnStructField("MaxPriceAttoFIL", "Int", true, true),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))

	ts.Accumulate(schema.SpawnStruct("StorageTask", []schema.StructField{
		schema.SpawnStructField("Miner", "String", false, false),
		schema.SpawnStructField("MaxPriceAttoFIL", "Int", false, false),
		schema.SpawnStructField("Size", "Int", false, false),
		schema.SpawnStructField("StartOffset", "Int", false, false),
		schema.SpawnStructField("FastRetrieval", "Bool", false, false),
		schema.SpawnStructField("Verified", "Bool", false, false),
		schema.SpawnStructField("Schedule", "String", true, true),
		schema.SpawnStructField("ScheduleLimit", "String", true, true),
		schema.SpawnStructField("Tag", "String", true, true),
		schema.SpawnStructField("RetrievalSchedule", "String", true, true),
		schema.SpawnStructField("RetrievalScheduleLimit", "String", true, true),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))

	ts.Accumulate(schema.SpawnStruct("Task", []schema.StructField{
		schema.SpawnStructField("UUID", "String", false, false),
		schema.SpawnStructField("Status", "Status", false, false),
		schema.SpawnStructField("WorkedBy", "String", true, false),
		schema.SpawnStructField("Stage", "String", false, false),
		schema.SpawnStructField("CurrentStageDetails", "StageDetails", true, false),
		schema.SpawnStructField("PastStageDetails", "List_StageDetails", true, false),
		schema.SpawnStructField("StartedAt", "Time", true, false),
		schema.SpawnStructField("RunCount", "Int", false, false),
		schema.SpawnStructField("ErrorMessage", "String", true, false),
		schema.SpawnStructField("RetrievalTask", "RetrievalTask", true, false),
		schema.SpawnStructField("StorageTask", "StorageTask", true, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))
	ts.Accumulate(schema.SpawnList("Tasks", "Task", false))

	ts.Accumulate(schema.SpawnStruct("FinishedTask", []schema.StructField{
		schema.SpawnStructField("Status", "Status", false, false),
		schema.SpawnStructField("StartedAt", "Time", false, false),
		schema.SpawnStructField("ErrorMessage", "String", true, false),
		schema.SpawnStructField("RetrievalTask", "RetrievalTask", true, true),
		schema.SpawnStructField("StorageTask", "StorageTask", true, true),
		schema.SpawnStructField("DealID", "Int", false, false),
		schema.SpawnStructField("MinerMultiAddr", "String", false, false),
		schema.SpawnStructField("ClientApparentAddr", "String", false, false),
		schema.SpawnStructField("MinerLatencyMS", "Int", true, true),
		schema.SpawnStructField("TimeToFirstByteMS", "Int", true, true),
		schema.SpawnStructField("TimeToLastByteMS", "Int", true, true),
		schema.SpawnStructField("Events", "Link_List_StageDetails", false, false),
		schema.SpawnStructField("MinerVersion", "String", true, true),
		schema.SpawnStructField("ClientVersion", "String", true, true),
		schema.SpawnStructField("Size", "Int", true, true),
		schema.SpawnStructField("PayloadCID", "String", true, true),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))
	ts.Accumulate(schema.SpawnList("FinishedTasks", "FinishedTask", false))
	ts.Accumulate(schema.SpawnLinkReference("Link_FinishedTask", "FinishedTask"))

	// client api
	ts.Accumulate(schema.SpawnStruct("PopTask", []schema.StructField{
		schema.SpawnStructField("Status", "Status", false, false),
		schema.SpawnStructField("WorkedBy", "String", false, false),
		schema.SpawnStructField("Tags", "List_String", true, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))
	ts.Accumulate(schema.SpawnStruct("UpdateTask", []schema.StructField{
		schema.SpawnStructField("Status", "Status", false, false),
		schema.SpawnStructField("ErrorMessage", "String", true, false),
		schema.SpawnStructField("Stage", "String", true, false),
		schema.SpawnStructField("CurrentStageDetails", "StageDetails", true, false),
		schema.SpawnStructField("WorkedBy", "String", false, false),
		schema.SpawnStructField("RunCount", "Int", false, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))

	// reputation api
	ts.Accumulate(schema.SpawnStruct("AuthenticatedRecord", []schema.StructField{
		schema.SpawnStructField("Record", "Link_FinishedTask", false, false),
		schema.SpawnStructField("Signature", "Bytes", false, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))
	ts.Accumulate(schema.SpawnList("List_AuthenticatedRecord", "AuthenticatedRecord", false))
	ts.Accumulate(schema.SpawnStruct("RecordUpdate", []schema.StructField{
		schema.SpawnStructField("Records", "List_AuthenticatedRecord", false, false),
		schema.SpawnStructField("SigPrev", "Bytes", false, false),
		schema.SpawnStructField("Previous", "Link", false, true),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))

	if errs := ts.ValidateGraph(); errs != nil {
		for _, err := range errs {
			fmt.Printf("- %s\n", err)
		}
		panic("not happening")
	}

	gengo.Generate(os.Args[1], "tasks", ts, adjCfg)
	gengraphql.Generate(path.Join(os.Args[1], "..", "controller", "graphql"), "graphql", ts, "tasks", "github.com/filecoin-project/dealbot/tasks")
}
