package commands

import (
	"fmt"

	"github.com/c2h5oh/datasize"
	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"
)

var MakeStorageDealCmd = &cli.Command{
	Name:   "storage-deal",
	Usage:  "Make storage deals with provided miners.",
	Flags:  append(SingleTaskFlags, StorageFlags...),
	Action: makeStorageDeal,
}

func makeStorageDeal(cctx *cli.Context) error {
	nodeConfig, node, closer, err := lotus.SetupClientFromCLI(cctx)
	if err != nil {
		return err
	}
	defer closer()

	// Read size parameter and interpret as file size (pre-piece padding)
	sizeHuman := cctx.String("size")
	var size datasize.ByteSize
	err = size.UnmarshalText([]byte(sizeHuman))
	if err != nil {
		return fmt.Errorf("size is not a recognizable byte size: %s, %s", sizeHuman, err)
	}

	// parse parameters that don't need validation
	miner := cctx.String("miner")
	verified := cctx.Bool("verified-deal")
	fastRetrieval := cctx.Bool("fast-retrieval")
	startOffset := cctx.Uint64("start-offset")
	maxPrice := cctx.Uint64("max-price")

	task := tasks.Type.StorageTask.Of(miner, int64(maxPrice), int64(size.Bytes()), int64(startOffset), fastRetrieval, verified, "")

	return tasks.MakeStorageDeal(cctx.Context, nodeConfig, node, task, emptyUpdateStage, log.Infow)
}
