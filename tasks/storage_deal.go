package tasks

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/google/uuid"
	"github.com/ipld/go-ipld-prime/schema"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

const maxPriceDefault = 5e16
const startOffsetDefault = 30760

func MakeStorageDeal(ctx context.Context, config NodeConfig, node api.FullNode, task StorageTask, updateStage UpdateStage, log LogStatus) error {
	de := &storageDealExecutor{
		dealExecutor: dealExecutor{
			ctx:    ctx,
			config: config,
			node:   node,
			miner:  task.Miner.x,
			log:    log,
		},
		task: task,
	}
	err := executeStage("MinerOnline", updateStage, []step{
		{de.getTipSet, "Tipset successfully fetched"},
		{de.getMinerInfo, "Miner Info successfully fetched"},
		{de.getPeerAddr, "Miner address validated"},
		{de.netConnect, "Connected to miner"},
	})
	if err != nil {
		return err
	}
	err = executeStage("QueryAsk", updateStage, []step{
		{de.queryAsk, "Successfully queried ask from miner"},
	})
	if err != nil {
		return err
	}
	err = executeStage("CheckPrice", updateStage, []step{
		{de.checkPrice, "Deal Price Validated"},
	})
	if err != nil {
		return err
	}
	err = executeStage("ClientImport", updateStage, []step{
		{de.generateFile, "Data file generated"},
		{de.importFile, "Data file imported to Lotus"},
	})
	if err != nil {
		return err
	}
	return de.executeAndMonitorDeal(updateStage)
}

type dealExecutor struct {
	ctx          context.Context
	config       NodeConfig
	node         api.FullNode
	miner        string
	log          LogStatus
	tipSet       *types.TipSet
	minerAddress address.Address
	minerInfo    miner.MinerInfo
	pi           peer.AddrInfo
}

type storageDealExecutor struct {
	dealExecutor
	task      StorageTask
	price     big.Int
	fileName  string
	importRes *api.ImportRes
}

func (de *dealExecutor) getTipSet() (err error) {
	de.tipSet, err = de.node.ChainHead(de.ctx)
	return err
}

func (de *dealExecutor) getMinerInfo() (err error) {
	// retrieve and validate miner price
	de.minerAddress, err = address.NewFromString(de.miner)
	if err != nil {
		return err
	}

	de.minerInfo, err = de.node.StateMinerInfo(de.ctx, de.minerAddress, de.tipSet.Key())
	return err
}

func (de *dealExecutor) getPeerAddr() error {
	if de.minerInfo.PeerId == nil {
		return fmt.Errorf("no PeerID for miner")
	}
	multiaddrs := make([]multiaddr.Multiaddr, 0, len(de.minerInfo.Multiaddrs))
	for i, a := range de.minerInfo.Multiaddrs {
		maddr, err := multiaddr.NewMultiaddrBytes(a)
		if err != nil {
			de.log("parsing multiaddr", "index", i, "address", a, "error", err)
			continue
		}
		multiaddrs = append(multiaddrs, maddr)
	}

	de.pi = peer.AddrInfo{
		ID:    *de.minerInfo.PeerId,
		Addrs: multiaddrs,
	}
	return nil
}

func (de *dealExecutor) netConnect() error {
	return de.node.NetConnect(de.ctx, de.pi)
}

func (de *storageDealExecutor) queryAsk() (err error) {
	ask, err := de.node.ClientQueryAsk(de.ctx, *de.minerInfo.PeerId, de.minerAddress)
	if err != nil {
		return err
	}

	if de.task.Verified.x {
		de.price = ask.VerifiedPrice
	} else {
		de.price = ask.Price
	}
	return
}

func (de *storageDealExecutor) checkPrice() error {
	maxPrice := abi.NewTokenAmount(de.task.MaxPriceAttoFIL.x)
	if de.task.MaxPriceAttoFIL.x == 0 {
		maxPrice = abi.NewTokenAmount(maxPriceDefault)
	}
	if de.price.GreaterThan(maxPrice) {
		return fmt.Errorf("miner ask price (%v) exceeds max price (%v)", de.price, maxPrice)
	}
	return nil
}

func (de *storageDealExecutor) generateFile() error {
	de.fileName = uuid.New().String()
	filePath := filepath.Join(de.config.DataDir, de.fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	de.log("creating deal file", "name", de.fileName, "size", de.task.Size, "path", filePath)
	_, err = io.CopyN(file, rand.Reader, de.task.Size.x)
	if err != nil {
		return fmt.Errorf("error creating random file for deal: %s, %v", de.fileName, err)
	}
	return nil
}

func (de *storageDealExecutor) importFile() (err error) {
	// import the file into the lotus node
	ref := api.FileRef{
		Path:  filepath.Join(de.config.NodeDataDir, de.fileName),
		IsCAR: false,
	}

	de.importRes, err = de.node.ClientImport(de.ctx, ref)
	if err != nil {
		return fmt.Errorf("error importing file: %w", err)
	}
	return nil
}

func (de *storageDealExecutor) executeAndMonitorDeal(updateStage UpdateStage) error {

	startOffset := de.task.StartOffset.x
	if startOffset == 0 {
		startOffset = startOffsetDefault
	}

	de.log("imported deal file, got data cid", "datacid", de.importRes.Root)

	// Prepare parameters for deal
	params := &api.StartDealParams{
		Data: &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         de.importRes.Root,
		},
		Wallet:            de.config.WalletAddress,
		Miner:             de.minerAddress,
		EpochPrice:        de.price,
		MinBlocksDuration: 2880 * 180,
		DealStartEpoch:    de.tipSet.Height() + abi.ChainEpoch(startOffset),
		FastRetrieval:     de.task.FastRetrieval.x,
		VerifiedDeal:      de.task.Verified.x,
	}
	_ = params

	// start deal process
	proposalCid, err := de.node.ClientStartDeal(de.ctx, params)
	if err != nil {
		return err
	}

	de.log("got proposal cid", "cid", proposalCid)

	// track updates to deal
	updates, err := de.node.ClientGetDealUpdates(de.ctx)
	if err != nil {
		return err
	}

	de.log("got deal updates channel")

	lastState := storagemarket.StorageDealUnknown
	for info := range updates {
		if !proposalCid.Equals(info.ProposalCid) {
			continue
		}
		stage := info.DealStages.GetStage(storagemarket.DealStates[info.State])
		if stage != nil {
			err = updateStage(storagemarket.DealStates[info.State], toStageDetails(stage))
			if err != nil {
				return err
			}
		}
		if info.State != lastState {
			de.log("Deal status",
				"cid", info.ProposalCid,
				"piece", info.PieceCID,
				"state", storagemarket.DealStates[info.State],
				"message", info.Message,
				"provider", info.Provider,
			)
			lastState = info.State
		}

		switch info.State {
		case storagemarket.StorageDealUnknown,
			storagemarket.StorageDealProposalNotFound,
			storagemarket.StorageDealProposalRejected,
			storagemarket.StorageDealExpired,
			storagemarket.StorageDealSlashed,
			storagemarket.StorageDealRejecting,
			storagemarket.StorageDealFailing,
			storagemarket.StorageDealError:

			logStages(info, de.log)
			return errors.New("storage deal failed")

		// deal is on chain, exit successfully
		case storagemarket.StorageDealActive:

			logStages(info, de.log)
			return nil
		}
	}

	return nil
}

func mktime(t time.Time) _Time {
	return _Time{x: t.UnixNano()}
}

func asStrM(s string) _String__Maybe {
	return _String__Maybe{m: schema.Maybe_Value, v: &_String{s}}
}

func toStageDetails(stage *storagemarket.DealStage) StageDetails {
	logs := make([]_Logs, 0, len(stage.Logs))
	for _, log := range stage.Logs {
		l := _Logs{
			Log:       _String{log.Log},
			UpdatedAt: mktime(log.UpdatedTime.Time()),
		}
		logs = append(logs, l)
	}
	detailTime := mktime(stage.UpdatedTime.Time())
	return &_StageDetails{
		Description:      asStrM(stage.Description),
		ExpectedDuration: asStrM(stage.ExpectedDuration),
		UpdatedAt:        _Time__Maybe{m: schema.Maybe_Value, v: &detailTime},
		Logs:             _List_Logs{logs},
	}
}

func logStages(info api.DealInfo, log LogStatus) {
	if info.DealStages == nil {
		log("Deal stages is nil")
		return
	}

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
}

func (sp *_StorageTask__Prototype) Of(miner string, maxPrice, size, startOffset int64, fastRetrieval, verified bool) StorageTask {
	st := _StorageTask{
		Miner:           _String{miner},
		MaxPriceAttoFIL: _Int{maxPrice},
		Size:            _Int{size},
		StartOffset:     _Int{startOffset},
		FastRetrieval:   _Bool{fastRetrieval},
		Verified:        _Bool{verified},
	}
	return &st
}
