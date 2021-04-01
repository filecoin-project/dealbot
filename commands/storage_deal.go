package commands

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/c2h5oh/datasize"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var MakeStorageDeal = &cli.Command{
	Name:  "storage-deal",
	Usage: "Make storage deals with provided miners.",
	Flags: []cli.Flag{
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
			Name:    "max-price",
			Usage:   "maximum Attofil to pay per byte per epoch []",
			EnvVars: []string{"DEALBOT_MAX_PRICE"},
			Value:   5e16,
		},
	},
	Action: makeStorageDeal,
}

func makeStorageDeal(cctx *cli.Context) error {
	if err := setupLogging(cctx); err != nil {
		return xerrors.Errorf("setup logging: %w", err)
	}

	// read dir and assert it exists
	dataDir := path.Dir(cctx.Path("data-dir"))
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data-dir does not exist: %s", dataDir)
	}
	nodeDataDir := path.Dir(cctx.Path("node-data-dir"))
	if nodeDataDir == "" {
		nodeDataDir = dataDir
	}

	// start API to lotus node
	opener, closer, err := setupLotusAPI(cctx)
	if err != nil {
		return err
	}
	defer closer()

	_ = opener

	node, jsoncloser, err := opener.Open(cctx.Context)
	if err != nil {
		return err
	}
	defer jsoncloser()

	// read addresses and assert they are addresses
	var walletAddress address.Address
	if cctx.IsSet("wallet") {
		walletParam := cctx.String("wallet")
		walletAddress, err = address.NewFromString(walletParam)
	} else {
		walletAddress, err = node.WalletDefaultAddress(context.Background())
	}
	if err != nil {
		return fmt.Errorf("wallet is not a Filecoin address: %s, %s", cctx.String("wallet"), err)
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

	fileName := uuid.New().String()
	file, err := os.Create(path.Join(dataDir, fileName))
	if err != nil {
		return err
	}
	_, err = io.CopyN(file, rand.Reader, int64(size))
	if err != nil {
		return fmt.Errorf("error creating random file for deal: %s, %v", fileName, err)
	}

	// import the file into the lotus node
	ref := api.FileRef{
		Path:  path.Join(nodeDataDir, fileName),
		IsCAR: true,
	}

	importRes, err := node.ClientImport(cctx.Context, ref)
	if err != nil {
		return err
	}

	pieceInfo, err := node.ClientDealPieceCID(cctx.Context, importRes.Root)
	if err != nil {
		return err
	}

	// Prepare parameters for deal
	params := &api.StartDealParams{
		Data: &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         importRes.Root,
			PieceCid:     &pieceInfo.PieceCID,
			PieceSize:    0,
			RawBlockSize: 0,
		},
		Wallet:            walletAddress,
		Miner:             minerAddress,
		EpochPrice:        price,
		MinBlocksDuration: 2880 * 180,
		//ProviderCollateral: ?, // TODO: configure? what's a good value?
		DealStartEpoch: tipSet.Height() + abi.ChainEpoch(startOffset),
		FastRetrieval:  fastRetrieval,
		VerifiedDeal:   verified,
	}
	_ = params

	// start deal process
	proposalCid, err := node.ClientStartDeal(cctx.Context, params)
	if err != nil {
		return err
	}

	// track updates to deal
	updates, err := node.ClientGetDealUpdates(cctx.Context)
	if err != nil {
		return err
	}

	for info := range updates {
		if proposalCid.Equals(info.ProposalCid) {
			log.Info(info)

			switch info.State {
			case storagemarket.StorageDealUnknown:
			case storagemarket.StorageDealProposalNotFound:
			case storagemarket.StorageDealProposalRejected:
			case storagemarket.StorageDealExpired:
			case storagemarket.StorageDealSlashed:
			case storagemarket.StorageDealRejecting:
			case storagemarket.StorageDealFailing:
			case storagemarket.StorageDealError:
				// deal failed, exit
				return nil
			case storagemarket.StorageDealActive:
				// deal is on chain, exit
				return nil
			}
		}
	}

	return nil
}

func minerAskPrice(ctx context.Context, api api.FullNode, tipSet *types.TipSet, addr address.Address) (abi.TokenAmount, error) {
	minerInfo, err := api.StateMinerInfo(ctx, addr, tipSet.Key())
	if err != nil {
		return big.Zero(), err
	}

	peerId := *minerInfo.PeerId
	ask, err := api.ClientQueryAsk(ctx, peerId, addr)
	if err != nil {
		return big.Zero(), err
	}

	return ask.Price, nil
}
