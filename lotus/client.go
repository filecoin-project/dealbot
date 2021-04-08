package lotus

import (
	"fmt"
	"os"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/go-address"
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

	//if err := setupLogging(cctx); err != nil {
	//return tasks.NodeConfig{}, nil, nil, xerrors.Errorf("setup logging: %w", err)
	//}

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
