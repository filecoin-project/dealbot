package commands

import (
	"context"
	"fmt"
	"github.com/c2h5oh/datasize"
	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
	"os"
	"path"
)

var MakeStorageDeal = &cli.Command{
	Name:  "MakeDeal",
	Usage: "Make storage deals with provided miners.",
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name:     "data-dir",
			Usage:    "writable directory used to transfer data to node",
			Aliases:  []string{"d"},
			EnvVars:  []string{"DEALBOT_DATA_DIRECTORY"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "wallet",
			Usage:    "deal client wallet address on node",
			Aliases:  []string{"w"},
			EnvVars:  []string{"DEALBOT_WALLET_ADDRESS"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "miner",
			Usage:    "address of miner receiving deal",
			Aliases:  []string{"m"},
			EnvVars:  []string{"DEALBOT_MINER_ADDRESS"},
			Required: true,
		},
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
			Usage:   "size of deal (1Kb, 2Mb, 12Gb, etc.) [1Mb]",
			Aliases: []string{"s"},
			EnvVars: []string{"DEALBOT_DEAL_SIZE"},
			Value:   "1Mb",
		},
		&cli.Int64Flag{
			Name:    "start-offset",
			Usage:   "epochs deal start will be offset from now [2880]",
			EnvVars: []string{"DEALBOT_START_OFFSET"},
			Value:   2888,
		},
		&cli.Int64Flag{
			Name:    "max-price",
			Usage:   "maximum Attofil to pay per byte per epoch []",
			EnvVars: []string{"DEALBOT_MAX_PRICE"},
			Value:   5e16,
		},
	},
	Action: makeDeal,
}

/*
Deal Steps:
 * Parse parameters
 * Get and verify ask price
 * Create file in shared directory
 * Import file with file ref
 * ClientCalcCommP to get piece cid
 * get current epoch and choose deal start.
 * Start deal
 * Read from deal status channel until exit
*/

func makeDeal(cctx *cli.Context) error {
	if err := setupLogging(cctx); err != nil {
		return xerrors.Errorf("setup logging: %w", err)
	}

	// read dir and assert it exists
	dataDir := path.Dir(cctx.Path("data-dir"))
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data-dir does not exist: %s", dataDir)
	}

	// read addresses and assert they are addresses
	walletParam := cctx.String("wallet")
	walletAddress, err := address.NewFromString(walletParam)
	if err != nil {
		return fmt.Errorf("wallet is not a Filecoin address: %s, %s", walletParam, err)
	}

	// get miner address
	minerParam := cctx.String("miner")
	minerAddress, err := address.NewFromString(minerParam)
	if err != nil {
		return fmt.Errorf("miner is not a Filecoint address: %s, %s", minerParam, err)
	}

	// Read size parameter and interpret as file size (pre-piece padding)
	sizeHuman := cctx.String("size")
	var size datasize.ByteSize
	err = size.UnmarshalText([]byte(sizeHuman))
	if err != nil {
		return fmt.Errorf("size is not a recognizable byte size: %s, %s", sizeHuman, err)
	}

	// parse parameters that don't need validation
	verified := cctx.Bool("verified-deal")
	fastRetrieval := cctx.Bool("fast-retrieval")
	startOffset := cctx.Int64("start-offset")
	maxPrice := abi.NewTokenAmount(cctx.Int64("max-price"))

	// start API to lotus node
	opener, closer, err := setupLotusAPI(cctx)
	if err != nil {
		return err
	}
	defer closer()

	_ = opener

	node, closer, err := opener.Open(cctx.Context)
	if err != nil {
		return err
	}

	// get chain head for chain queries and to get height
	tipSet, err := node.ChainHead(cctx.Context)
	if err != nil {
		return err
	}

	// retrieve and validate miner price
	price, err := minerAskPrice(cctx.Context, node, tipSet, minerAddress)
	if err != nil {
		return err
	}

	if price.GreaterThan(maxPrice) {
		return fmt.Errorf("miner ask price (%v) exceeds max price (%v)", price, maxPrice)
	}

	// TODO: create random car file of appropriate size
	ref := api.FileRef{
		//Path:  path_to_file,
		IsCAR: true,
	}

	importRes, err := node.Import(cctx.Context, ref)
	if err != nil {
		return err
	}

	// TODO: compute piece cid and piece size from file
	dataRef := &storagemarket.DataRef{
		//TransferType: "",
		Root: importRes.Root,
		//PieceCid:     ,
		PieceSize:    0,
		RawBlockSize: 0,
	}

	params := &api.StartDealParams{
		Data:       dataRef,
		Wallet:     walletAddress,
		Miner:      minerAddress,
		EpochPrice: price,
		//MinBlocksDuration:  ?,
		//ProviderCollateral: ?,
		DealStartEpoch: tipSet.Height() + abi.ChainEpoch(startOffset),
		FastRetrieval:  fastRetrieval,
		VerifiedDeal:   verified,
	}
	_ = params

	//node.MakeDeal(ctx, params)

	// TODO: track deal status and log results

	return nil
}

func minerAskPrice(ctx context.Context, api lotus.API, tipSet *types.TipSet, addr address.Address) (abi.TokenAmount, error) {
	minerInfo, err := api.MinerInfo(ctx, addr, tipSet.Key())
	if err != nil {
		return big.Zero(), err
	}

	peerId := *minerInfo.PeerId
	ask, err := api.QueryAsk(ctx, peerId, addr)
	if err != nil {
		return big.Zero(), err
	}

	return ask.Price, nil
}
