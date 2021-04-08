package lotus

import (
	"context"
	"fmt"
	"os"

	"github.com/filecoin-project/dealbot/config"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

type NodeCloser func()

func SetupClientFromCLI(cctx *cli.Context) (tasks.NodeConfig, api.FullNode, NodeCloser, error) {
	// read dir and assert it exists
	dataDir := cctx.String("data-dir")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return tasks.NodeConfig{}, nil, nil, fmt.Errorf("data-dir does not exist: %s", dataDir)
	}

	nodeDataDir := cctx.String("node-data-dir")
	if nodeDataDir == "" {
		nodeDataDir = dataDir
	}

	if err := setupLogging(cctx); err != nil {
		return tasks.NodeConfig{}, nil, nil, xerrors.Errorf("setup logging: %w", err)
	}

	// start API to lotus node
	opener, apiCloser, err := NewAPIOpenerFromCLI(cctx)
	if err != nil {
		return tasks.NodeConfig{}, nil, nil, err
	}

	node, jsoncloser, err := opener.Open(cctx.Context)
	if err != nil {
		apiCloser()
		return tasks.NodeConfig{}, nil, nil, err
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
		return tasks.NodeConfig{}, nil, nil, fmt.Errorf("wallet is not a Filecoin address: %s, %s", cctx.String("wallet"), err)
	}

	return tasks.NodeConfig{
		DataDir:       dataDir,
		NodeDataDir:   nodeDataDir,
		WalletAddress: walletAddress,
	}, node, closer, nil
}

func SetupClient(cfg *config.EnvConfig) (tasks.NodeConfig, api.FullNode, NodeCloser, error) {
	// read dir and assert it exists
	dataDir := cfg.Daemon.DataDir
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return tasks.NodeConfig{}, nil, nil, fmt.Errorf("data-dir does not exist: %s", dataDir)
	}

	nodeDataDir := cfg.Daemon.NodeDataDir
	if nodeDataDir == "" {
		nodeDataDir = dataDir
	}

	// start API to lotus node
	opener, apiCloser, err := NewAPIOpener(cfg)
	if err != nil {
		return tasks.NodeConfig{}, nil, nil, err
	}

	ctx := context.TODO()
	node, jsoncloser, err := opener.Open(ctx)
	if err != nil {
		apiCloser()
		return tasks.NodeConfig{}, nil, nil, err
	}

	closer := func() {
		apiCloser()
		jsoncloser()
	}

	// read addresses and assert they are addresses
	var walletAddress address.Address
	wallet := cfg.Daemon.Wallet
	if wallet != "" {
		log.Infow("using set wallet", wallet)
		walletAddress, err = address.NewFromString(wallet)
	} else {
		log.Infow("using default wallet")
		walletAddress, err = node.WalletDefaultAddress(ctx)
	}
	if err != nil {
		return tasks.NodeConfig{}, nil, nil, fmt.Errorf("wallet is not a Filecoin address: %s, %s", wallet, err)
	}

	return tasks.NodeConfig{
		DataDir:       dataDir,
		NodeDataDir:   nodeDataDir,
		WalletAddress: walletAddress,
	}, node, closer, nil
}
