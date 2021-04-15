package commands

import (
	_ "github.com/lib/pq"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("dealbot")

var CommonFlags []cli.Flag = []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "wallet",
		Usage:   "deal client wallet address on node",
		Aliases: []string{"w"},
		EnvVars: []string{"DEALBOT_WALLET_ADDRESS"},
	}),
}

var DealFlags = []cli.Flag{
	altsrc.NewPathFlag(&cli.PathFlag{
		Name:     "data-dir",
		Usage:    "writable directory used to transfer data to node",
		Aliases:  []string{"d"},
		EnvVars:  []string{"DEALBOT_DATA_DIRECTORY"},
		Required: true,
	}),
	altsrc.NewPathFlag(&cli.PathFlag{
		Name:    "node-data-dir",
		Usage:   "data-dir from relative to node's location [data-dir]",
		Aliases: []string{"n"},
		EnvVars: []string{"DEALBOT_NODE_DATA_DIRECTORY"},
	}),
}

var SingleTaskFlags = append(DealFlags, []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:     "miner",
		Usage:    "address of miner to make deal with",
		Aliases:  []string{"m"},
		EnvVars:  []string{"DEALBOT_MINER_ADDRESS"},
		Required: true,
	}),
}...)

var DaemonFlags = append(DealFlags, append(CommonFlags, []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "listen",
		Usage:   "host:port to bind http server on",
		Aliases: []string{"l"},
		EnvVars: []string{"DEALBOT_LISTEN"},
	}),
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:     "endpoint",
		Usage:    "HTTP endpoint of the controller",
		Aliases:  []string{"e"},
		EnvVars:  []string{"DEALBOT_CONTROLLER_ENDPOINT"},
		Required: true,
	}),
}...)...)

var ControllerFlags = []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "listen",
		Usage:   "host:port to bind http server on",
		Aliases: []string{"l"},
		EnvVars: []string{"DEALBOT_LISTEN"},
	}),
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "metrics",
		Usage:   "value of 'prometheus' or 'log'",
		Aliases: []string{"l"},
		EnvVars: []string{"DEALBOT_METRICS"},
	}),
}

var AllFlags = append(DealFlags, append(SingleTaskFlags, append(DaemonFlags, ControllerFlags...)...)...)
