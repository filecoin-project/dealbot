package commands

import (
	"fmt"

	"github.com/c2h5oh/datasize"
	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"
)

// QueueStorageCmd queues a storage deal with the controller (as opposed to executing directly with lotus)
var QueueStorageCmd = &cli.Command{
	Name:   "queue-storage",
	Usage:  "Queue storage deal in the controller",
	Flags:  append(append(EndpointFlags, MinerFlags...), StorageFlags...),
	Action: queueStorageDeal,
}

func queueStorageDeal(cctx *cli.Context) error {
	client := client.New(cctx)

	// Read size parameter and interpret as file size (pre-piece padding)
	sizeHuman := cctx.String("size")
	var size datasize.ByteSize
	err := size.UnmarshalText([]byte(sizeHuman))
	if err != nil {
		return fmt.Errorf("size is not a recognizable byte size: %s, %s", sizeHuman, err)
	}

	// parse parameters that don't need validation
	miner := cctx.String("miner")
	verified := cctx.Bool("verified-deal")
	fastRetrieval := cctx.Bool("fast-retrieval")
	startOffset := cctx.Uint64("start-offset")
	maxPrice := cctx.Uint64("max-price")

	task := tasks.NewStorageTask(miner, int64(maxPrice), int64(size.Bytes()), int64(startOffset), fastRetrieval, verified, "")

	t, err := client.CreateStorageTask(cctx.Context, task)

	if err != nil {
		log.Fatal(err)
	}

	log.Infof("queued storage deal with UUID: %s", t.UUID)

	return nil
}
