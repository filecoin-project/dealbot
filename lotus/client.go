package lotus

import (
	"context"
	"fmt"
	"os"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/urfave/cli/v2"
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
		return tasks.NodeConfig{}, nil, nil, fmt.Errorf("setup logging: %w", err)
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
	mf := big.NewInt(100000000000000)
	if cctx.IsSet("minfil") {
		log.Infow("using minimum wallet fil for picking tasks", cctx.String("minfil"))
		if _, ok := mf.SetString(cctx.String("minfil"), 0); !ok {
			return tasks.NodeConfig{}, nil, nil, fmt.Errorf("could not parse min wallet balance: %s, %s", cctx.String("minfil"), err)
		}
	}
	mc := big.NewInt(0)
	if cctx.IsSet("mincap") {
		log.Infow("using minimum wallet datacap for picking tasks", cctx.String("mincap"))
		if _, ok := mf.SetString(cctx.String("mincap"), 0); !ok {
			return tasks.NodeConfig{}, nil, nil, fmt.Errorf("could not parse min wallet datacap: %s, %s", cctx.String("mincap"), err)
		}
	}

	return tasks.NodeConfig{
		DataDir:          dataDir,
		NodeDataDir:      nodeDataDir,
		WalletAddress:    walletAddress,
		MinWalletBalance: mf,
		MinWalletCap:     mc,
	}, node, closer, nil
}

func SetupClient(ctx context.Context, cliCtx *cli.Context) (tasks.NodeConfig, api.FullNode, NodeCloser, error) {
	// read dir and assert it exists
	dataDir := cliCtx.String("data-dir")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return tasks.NodeConfig{}, nil, nil, fmt.Errorf("data-dir does not exist: %s", dataDir)
	}

	nodeDataDir := cliCtx.String("node-data-dir")
	if nodeDataDir == "" {
		nodeDataDir = dataDir
	}

	// start API to lotus node
	opener, apiCloser, err := NewAPIOpener(cliCtx)
	if err != nil {
		return tasks.NodeConfig{}, nil, nil, err
	}

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
	wallet := cliCtx.String("wallet")
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
	mf := big.NewInt(100000000000000)
	if cliCtx.IsSet("minfil") {
		log.Infow("using minimum wallet fil for picking tasks", cliCtx.String("minfil"))
		if _, ok := mf.SetString(cliCtx.String("minfil"), 0); !ok {
			return tasks.NodeConfig{}, nil, nil, fmt.Errorf("could not parse min wallet balance: %s, %s", cliCtx.String("minfil"), err)
		}
	}
	mc := big.NewInt(0)
	if cliCtx.IsSet("mincap") {
		log.Infow("using minimum wallet datacap for picking tasks", cliCtx.String("mincap"))
		if _, ok := mf.SetString(cliCtx.String("mincap"), 0); !ok {
			return tasks.NodeConfig{}, nil, nil, fmt.Errorf("could not parse min wallet datacap: %s, %s", cliCtx.String("mincap"), err)
		}
	}

	return tasks.NodeConfig{
		DataDir:          dataDir,
		NodeDataDir:      nodeDataDir,
		WalletAddress:    walletAddress,
		MinWalletBalance: mf,
		MinWalletCap:     mc,
	}, node, closer, nil
}
