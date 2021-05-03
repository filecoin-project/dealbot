package main

import (
	"fmt"
	"os"

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

	ts.Accumulate(schema.SpawnStruct("StageDetails", []schema.StructField{
		schema.SpawnStructField("Description", "String", true, false),
		schema.SpawnStructField("ExpectedDuration", "String", true, false),
		schema.SpawnStructField("Logs", "List_Logs", false, false),
		schema.SpawnStructField("UpdatedAt", "Time", true, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))
	ts.Accumulate(schema.SpawnLinkReference("Link_StageDetails", "StageDetails"))

	ts.Accumulate(schema.SpawnList("List_Logs", "Logs", false))
	ts.Accumulate(schema.SpawnStruct("Logs", []schema.StructField{
		schema.SpawnStructField("Log", "String", false, false),
		schema.SpawnStructField("UpdatedAt", "Time", false, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))

	ts.Accumulate(schema.SpawnStruct("RetrievalTask", []schema.StructField{
		schema.SpawnStructField("Miner", "String", false, false),
		schema.SpawnStructField("PayloadCID", "String", false, false),
		schema.SpawnStructField("CARExport", "Bool", false, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))
	ts.Accumulate(schema.SpawnLinkReference("Link_RetrievalTask", "RetrievalTask"))

	ts.Accumulate(schema.SpawnStruct("StorageTask", []schema.StructField{
		schema.SpawnStructField("Miner", "String", false, false),
		schema.SpawnStructField("MaxPriceAttoFIL", "Int", false, false),
		schema.SpawnStructField("Size", "Int", false, false),
		schema.SpawnStructField("StartOffset", "Int", false, false),
		schema.SpawnStructField("FastRetrieval", "Bool", false, false),
		schema.SpawnStructField("Verified", "Bool", false, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))
	ts.Accumulate(schema.SpawnLinkReference("Link_StorageTask", "StorageTask"))

	ts.Accumulate(schema.SpawnStruct("Task", []schema.StructField{
		schema.SpawnStructField("UUID", "String", false, false),
		schema.SpawnStructField("Status", "Status", false, false),
		schema.SpawnStructField("WorkedBy", "String", true, false),
		schema.SpawnStructField("Stage", "String", false, false),
		schema.SpawnStructField("CurrentStageDetails", "Link_StageDetails", true, false),
		schema.SpawnStructField("StartedAt", "Time", true, false),
		schema.SpawnStructField("RetrievalTask", "Link_RetrievalTask", true, false),
		schema.SpawnStructField("StorageTask", "Link_StorageTask", true, false),
	}, schema.SpawnStructRepresentationMap(map[string]string{})))

	if errs := ts.ValidateGraph(); errs != nil {
		for _, err := range errs {
			fmt.Printf("- %s\n", err)
		}
		panic("not happening")
	}

	gengo.Generate(os.Args[1], "tasks", ts, adjCfg)
}
