package commands

import (
	"strings"

	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/version"

	logging "github.com/ipfs/go-log/v2"
	_ "github.com/lib/pq"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var log = logging.Logger("dealbot")

func setupLogging(cctx *cli.Context) error {
	ll := cctx.String("log-level")
	if err := logging.SetLogLevel("*", ll); err != nil {
		return xerrors.Errorf("set log level: %w", err)
	}

	if err := logging.SetLogLevel("rpc", "error"); err != nil {
		return xerrors.Errorf("set rpc log level: %w", err)
	}

	llnamed := cctx.String("log-level-named")
	if llnamed == "" {
		return nil
	}

	for _, llname := range strings.Split(llnamed, ",") {
		parts := strings.Split(llname, ":")
		if len(parts) != 2 {
			return xerrors.Errorf("invalid named log level format: %q", llname)
		}
		if err := logging.SetLogLevel(parts[0], parts[1]); err != nil {
			return xerrors.Errorf("set named log level %q to %q: %w", parts[0], parts[1], err)
		}

	}

	log.Infof("Dealbot version:%s", version.String())

	return nil
}

func setupLotusAPI(cctx *cli.Context) (*lotus.APIOpener, lotus.APICloser, error) {
	return lotus.NewAPIOpener(cctx, 50_000)
}

var CommonFlags []cli.Flag = []cli.Flag{
	&cli.PathFlag{
		Name:    "data-dir",
		Usage:   "writable directory used to transfer data to node",
		Aliases: []string{"d"},
		EnvVars: []string{"DEALBOT_DATA_DIRECTORY"},
		//Required: true,
	},
	&cli.PathFlag{
		Name:    "node-data-dir",
		Usage:   "data-dir from relative to node's location [data-dir]",
		Aliases: []string{"n"},
		EnvVars: []string{"DEALBOT_NODE_DATA_DIRECTORY"},
	},
	&cli.StringFlag{
		Name:    "wallet",
		Usage:   "deal client wallet address on node",
		Aliases: []string{"w"},
		EnvVars: []string{"DEALBOT_WALLET_ADDRESS"},
	},
	&cli.StringFlag{
		Name:    "miner",
		Usage:   "address of miner to make deal with",
		Aliases: []string{"m"},
		EnvVars: []string{"DEALBOT_MINER_ADDRESS"},
		//Required: true,
	},
	&cli.Int64Flag{
		Name:    "start-offset",
		Usage:   "epochs deal start will be offset from now [5760 (2 days)]",
		EnvVars: []string{"DEALBOT_START_OFFSET"},
		Value:   30760,
	},
}
