package commands

import (
	"context"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"
)

var MakeRetrievalDealCmd = &cli.Command{
	Name:  "retrieval-deal",
	Usage: "Make retrieval deals with provided miners.",
	Flags: append(SingleTaskFlags, []cli.Flag{
		&cli.StringFlag{
			Name:  "cid",
			Usage: "payload cid to fetch from miner",
			Value: "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
		},
	}...),
	Action: makeRetrievalDeal,
}

func makeRetrievalDeal(cctx *cli.Context) error {
	clientConfig, node, closer, err := setupCLIClient(cctx)
	if err != nil {
		return err
	}
	defer closer()

	v, err := node.Version(context.Background())
	if err != nil {
		return err
	}

	log.Infof("remote version: %s", v.Version)

	carExport := true
	payloadCid := cctx.String("cid")

	log.Infof("retrieving cid: %s", payloadCid)

	// get miner address
	minerParam := cctx.String("miner")

	task := tasks.RetrievalTask{
		Miner:      minerParam,
		PayloadCID: payloadCid,
		CarExport:  carExport,
	}

	err = tasks.MakeRetrievalDeal(cctx.Context, clientConfig, node, task, func(msg string, keysAndValues ...interface{}) {
		log.Infow(msg, keysAndValues...)
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Info("successfully retrieved")

	return nil
}
