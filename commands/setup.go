package commands

import (
	"fmt"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"os"
	"strings"

	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/version"

	logging "github.com/ipfs/go-log/v2"
	_ "github.com/lib/pq"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var log = logging.Logger("dealbot")

type NodeCloser func()

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

func setupCLIClient(cctx *cli.Context) (tasks.ClientConfig, api.FullNode, NodeCloser, error) {
	// read dir and assert it exists
	dataDir := cctx.String("data-dir")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return tasks.ClientConfig{}, nil, nil, fmt.Errorf("data-dir does not exist: %s", dataDir)
	}

	nodeDataDir := cctx.String("node-data-dir")
	if nodeDataDir == "" {
		nodeDataDir = dataDir
	}

	if err := setupLogging(cctx); err != nil {
		return tasks.ClientConfig{}, nil, nil, xerrors.Errorf("setup logging: %w", err)
	}

	// start API to lotus node
	opener, apiCloser, err := setupLotusAPI(cctx)
	if err != nil {
		return tasks.ClientConfig{}, nil, nil, err
	}

	node, jsoncloser, err := opener.Open(cctx.Context)
	if err != nil {
		apiCloser()
		return tasks.ClientConfig{}, nil, nil, err
	}

	closer := func() {
		apiCloser()
		jsoncloser()
	}

	// read addresses and assert they are addresses
	var walletAddress address.Address
	if cctx.IsSet("wallet") {
		log.Infow("using set wallet", cctx.String("wallet"))
		walletAddress, err = address.NewFromString(cctx.String("wallet"))
	} else {
		log.Infow("using default wallet")
		walletAddress, err = node.WalletDefaultAddress(cctx.Context)
	}
	if err != nil {
		return tasks.ClientConfig{}, nil, nil, fmt.Errorf("wallet is not a Filecoin address: %s, %s", cctx.String("wallet"), err)
	}

	return tasks.ClientConfig{
		DataDir:       dataDir,
		NodeDataDir:   nodeDataDir,
		WalletAddress: walletAddress,
	}, node, closer, nil
}
