package tasks

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/google/uuid"
)

type StorageTask struct {
	Miner           string `json:"miner"`
	MaxPriceAttoFIL uint64 `json:"max_price_attofil"`
	Size            uint64 `json:"size"`
	StartOffset     uint64 `json:"start_offset"`
	FastRetrieval   bool   `json:"fast_retrieval"`
	Verified        bool   `json:"verified"`
}

const maxPriceDefault = 5e16
const startOffsetDefault = 30760

func MakeStorageDeal(ctx context.Context, config NodeConfig, node api.FullNode, task StorageTask, log UpdateStatus) error {
	// get chain head for chain queries and to get height
	tipSet, err := node.ChainHead(ctx)
	if err != nil {
		return err
	}

	// retrieve and validate miner price
	minerAddress, err := address.NewFromString(task.Miner)
	if err != nil {
		return err
	}
	price, err := minerAskPrice(ctx, node, tipSet, minerAddress)
	if err != nil {
		return err
	}

	maxPrice := abi.NewTokenAmount(int64(task.MaxPriceAttoFIL))
	if task.MaxPriceAttoFIL == 0 {
		maxPrice = abi.NewTokenAmount(maxPriceDefault)
	}
	if price.GreaterThan(maxPrice) {
		return fmt.Errorf("miner ask price (%v) exceeds max price (%v)", price, maxPrice)
	}

	fileName := uuid.New().String()
	filepath := path.Join(config.DataDir, fileName)
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	log("creating deal file", "name", fileName, "size", task.Size, "path", filepath)
	_, err = io.CopyN(file, rand.Reader, int64(task.Size))
	if err != nil {
		return fmt.Errorf("error creating random file for deal: %s, %v", fileName, err)
	}

	// import the file into the lotus node
	ref := api.FileRef{
		Path:  path.Join(config.NodeDataDir, fileName),
		IsCAR: false,
	}

	importRes, err := node.ClientImport(ctx, ref)
	if err != nil {
		return err
	}

	startOffset := task.StartOffset
	if startOffset == 0 {
		startOffset = startOffsetDefault
	}

	log("imported deal file, got data cid", "datacid", importRes.Root)

	// Prepare parameters for deal
	params := &api.StartDealParams{
		Data: &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         importRes.Root,
		},
		Wallet:            config.WalletAddress,
		Miner:             minerAddress,
		EpochPrice:        price,
		MinBlocksDuration: 2880 * 180,
		DealStartEpoch:    tipSet.Height() + abi.ChainEpoch(startOffset),
		FastRetrieval:     task.FastRetrieval,
		VerifiedDeal:      task.Verified,
	}
	_ = params

	// start deal process
	proposalCid, err := node.ClientStartDeal(ctx, params)
	if err != nil {
		return err
	}

	// track updates to deal
	updates, err := node.ClientGetDealUpdates(ctx)
	if err != nil {
		return err
	}

	lastState := storagemarket.StorageDealUnknown
	for info := range updates {
		if proposalCid.Equals(info.ProposalCid) {
			if info.State != lastState {
				log("Deal status",
					"cid", info.ProposalCid,
					"piece", info.PieceCID,
					"state", info.State,
					"message", info.Message,
					"provider", info.Provider,
				)
				lastState = info.State
			}

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
			case storagemarket.StorageDealActive:
				// deal is on chain, exit
				for _, stage := range info.DealStages.Stages {
					log("Deal stage",
						"cid", info.ProposalCid,
						"name", stage.Name,
						"description", stage.Description,
						"created", stage.CreatedTime,
						"expected", stage.ExpectedDuration,
						"updated", stage.UpdatedTime,
						"size", info.Size,
						"duration", info.Duration,
						"deal_id", info.DealID,
						"piece_cid", info.PieceCID,
						"message", info.Message,
						"provider", info.Provider,
						"price", info.PricePerEpoch,
						"verfied", info.Verified,
					)
				}
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
