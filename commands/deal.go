package commands

import (
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/lotus/api"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var MakeStorageDeals = &cli.Command{
	Name:  "MakeDeals",
	Usage: "Make storage deals with provided miners.",
	Flags: []cli.Flag{
		// TODO: parameters?
	},
	Action: makeDeals,
}

func makeDeals(cctx *cli.Context) error {
	// Validate flags
	heightFrom := cctx.Int64("from")
	heightTo := cctx.Int64("to")

	if heightFrom > heightTo {
		return xerrors.Errorf("--from must not be greater than --to")
	}

	if err := setupLogging(cctx); err != nil {
		return xerrors.Errorf("setup logging: %w", err)
	}

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
		Data: dataRef,
		//Wallet:             from,
		//Miner:              miner,
		//EpochPrice:         ?,
		//MinBlocksDuration:  ?,
		//ProviderCollateral: ?,
		//DealStartEpoch:     ?,
		//FastRetrieval:      ?,
		//VerifiedDeal:       ?,
	}
	_ = params

	//node.MakeDeal(ctx, params)

	// TODO: track deal status and log results

	return nil
}
