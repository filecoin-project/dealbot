package commands

import (
	_ "github.com/lib/pq"
	"github.com/urfave/cli/v2"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("dealbot")

var CommonFlags []cli.Flag = []cli.Flag{
	&cli.StringFlag{
		Name:    "wallet",
		Usage:   "deal client wallet address on node",
		Aliases: []string{"w"},
		EnvVars: []string{"DEALBOT_WALLET_ADDRESS"},
	},
}

var DealFlags = []cli.Flag{
	&cli.PathFlag{
		Name:     "data-dir",
		Usage:    "writable directory used to transfer data to node",
		Aliases:  []string{"d"},
		EnvVars:  []string{"DEALBOT_DATA_DIRECTORY"},
		Required: true,
	},
	&cli.PathFlag{
		Name:    "node-data-dir",
		Usage:   "data-dir from relative to node's location [data-dir]",
		Aliases: []string{"n"},
		EnvVars: []string{"DEALBOT_NODE_DATA_DIRECTORY"},
	},
}

var SingleTaskFlags = append(DealFlags, []cli.Flag{
	&cli.StringFlag{
		Name:     "miner",
		Usage:    "address of miner to make deal with",
		Aliases:  []string{"m"},
		EnvVars:  []string{"DEALBOT_MINER_ADDRESS"},
		Required: true,
	},
}...)
