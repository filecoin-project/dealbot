package commands

import (
	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"
)

// QueueRetrievalCmd queues a retrieval with the controller (as opposed to executing directly with lotus)
var QueueRetrievalCmd = &cli.Command{
	Name:   "queue-retrieval",
	Usage:  "Queue retrieval deal in the controller",
	Flags:  append(EndpointFlags, RetrievalFlags...),
	Action: queueRetrievalDeal,
}

func queueRetrievalDeal(cctx *cli.Context) error {
	client := client.New(cctx)
	carExport := false
	payloadCid := cctx.String("cid")

	log.Infof("retrieving cid: %s", payloadCid)

	// get miner address
	minerParam := cctx.String("miner")

	task := tasks.RetrievalTask{
		Miner:      minerParam,
		PayloadCID: payloadCid,
		CARExport:  carExport,
	}

	t, err := client.CreateRetrievalTask(cctx.Context, &task)

	if err != nil {
		log.Fatal(err)
	}

	log.Infof("queued retrieval deal with UUID: %s", t.UUID)

	return nil

}
