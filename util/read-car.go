// Compile with:  go build -o read-car read-car.go
// Run:           ./read-car out.car

package main

import (
	"fmt"
	"os"
	"io"
	"io/ioutil"
	"bytes"
	"encoding/json"
	"time"

	car "github.com/ipld/go-car"
)

func usage() {
	fmt.Printf("Usage:  %s CAR_FILE\n", os.Args[0])
}

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(1)
	}
	carFilePath := os.Args[1]
	carBytes, err := ioutil.ReadFile(carFilePath)
	if err != nil {
		panic(err)
	}
	cr, err := car.NewCarReader(bytes.NewReader(carBytes))
	if err != nil {
		panic(err)
	}

	for {
		carBlock, err := cr.Next()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}
		blockDataBytes := carBlock.RawData()
		blockDataStr := string(blockDataBytes)
		//fmt.Printf("%+v\n",blk)
		//fmt.Printf("%#v\n",carBlock)
		//fmt.Printf("---\n",carBlock)
		//fmt.Printf("block cid='%s'\n",.carBlock.Cid()String())
		//fmt.Printf("\n\n\n'%s'\n", blockDataStr)

		// Weird edge case where json is just "[]\n"
		if (blockDataStr=="[]\n") {
			continue
		}

		//var dealbotData map[string]interface{}
		var dealbotData interface{}
		json.Unmarshal(blockDataBytes, &dealbotData)
		dealbotJsonDataMap := dealbotData.(map[string]interface{})

		// skip the first block of signatures
		_, isRetrievalJsonObj := dealbotJsonDataMap["RetrievalTask"]
		if (!isRetrievalJsonObj) {
			continue
		}


		//dealbotJsonDataMap := dealbotData[""].(map[string]interface{})
		status := int(dealbotJsonDataMap["Status"].(float64))
		var statusStr string
		switch (status) {
		case 1:
			statusStr = "2 (queued)"
		case 2:
			statusStr = "2 (in progress)"
		case 3:
			statusStr = "3 (success)"
		case 4:
			statusStr = "4 (failed)"
		default:
			statusStr = "failed"
		}
		startDatetime := time.Unix(0, int64(dealbotJsonDataMap["StartedAt"].(float64)))
		retrievalTasksMap := dealbotJsonDataMap["RetrievalTask"].(map[string]interface{})
		payloadCid := retrievalTasksMap["PayloadCID"]
		miner := retrievalTasksMap["Miner"]

		// final output
		fmt.Printf("status = %s\n",statusStr)
		fmt.Printf("datetime = %s\n", startDatetime.String())
		fmt.Printf("ipfs location = https://ipfs.io/ipfs/%s\n",payloadCid)
		fmt.Printf("miner = %s\n",miner)
		fmt.Printf("\n")
	}
}
