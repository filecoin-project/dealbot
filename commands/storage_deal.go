package commands

import (
	"fmt"

	"github.com/c2h5oh/datasize"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"
)

var MakeStorageDealCmd = &cli.Command{
	Name:  "storage-deal",
	Usage: "Make storage deals with provided miners.",
	Flags: append(SingleTaskFlags, []cli.Flag{
		&cli.BoolFlag{
			Name:    "fast-retrieval",
			Usage:   "request fast retrieval [true]",
			Aliases: []string{"f"},
			EnvVars: []string{"DEALBOT_FAST_RETRIEVAL"},
			Value:   true,
		},
		&cli.BoolFlag{
			Name:    "verified-deal",
			Usage:   "true if deal is verified [false]",
			Aliases: []string{"v"},
			EnvVars: []string{"DEALBOT_VERIFIED_DEAL"},
			Value:   false,
		},
		&cli.StringFlag{
			Name:    "size",
			Usage:   "size of deal (1KB, 2MB, 12GB, etc.) [1MB]",
			Aliases: []string{"s"},
			EnvVars: []string{"DEALBOT_DEAL_SIZE"},
			Value:   "1MB",
		},
		&cli.Int64Flag{
			Name:    "max-price",
			Usage:   "maximum Attofil to pay per byte per epoch []",
			EnvVars: []string{"DEALBOT_MAX_PRICE"},
			Value:   5e16,
		},
		&cli.Int64Flag{
			Name:    "start-offset",
			Usage:   "epochs deal start will be offset from now [5760 (2 days)]",
			EnvVars: []string{"DEALBOT_START_OFFSET"},
			Value:   30760,
		},
	}...),
	Action: makeStorageDeal,
}

func makeStorageDeal(cctx *cli.Context) error {
	clientConfig, node, closer, err := setupCLIClient(cctx)
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

	task := tasks.StorageTask{
		Miner:           miner,
		MaxPriceAttoFIL: maxPrice,
		Size:            size.Bytes(),
		StartOffset:     startOffset,
		FastRetrieval:   fastRetrieval,
		Verified:        verified,
	}

	return tasks.MakeStorageDeal(cctx.Context, clientConfig, node, task, func(msg string, keysAndValues ...interface{}) {
		log.Infow(msg, keysAndValues...)
	})
}
