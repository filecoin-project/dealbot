package tasks

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/filecoin-project/dealbot/netutil"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/google/uuid"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/config"
	"github.com/multiformats/go-multiaddr"
)

var StorageDealStages = func() []string {
	ret := make([]string, 0, len(storagemarket.DealStates))
	for _, v := range storagemarket.DealStates {
		ret = append(ret, v)
	}
	return ret
}

const (
	maxPriceDefault    = 5e16
	startOffsetDefault = 30760
)

func MakeStorageDeal(ctx context.Context, config NodeConfig, node api.FullNode, task StorageTask, updateStage UpdateStage, log LogStatus, stageTimeouts map[string]time.Duration, releaseWorker func()) error {
	de := &storageDealExecutor{
		dealExecutor: dealExecutor{
			ctx:           ctx,
			config:        config,
			node:          node,
			miner:         task.Miner.x,
			log:           log,
			makeHost:      libp2p.New,
			releaseWorker: releaseWorker,
		},
		task: task,
	}

	defer func() {
		_ = de.cleanupDeal()
	}()

	defaultTimeout := stageTimeouts[defaultStorageStageTimeoutName]
	getStageCtx := func(stage string) (context.Context, context.CancelFunc) {
		timeout, ok := stageTimeouts[stage]
		if !ok {
			timeout = defaultTimeout
		}
		return context.WithTimeout(ctx, timeout)
	}

	stage := "MinerOnline"
	stageCtx, cancel := getStageCtx(stage)
	err := executeStage(stageCtx, stage, updateStage, []step{
		{de.getTipSet, "Tipset successfully fetched"},
		{de.getMinerInfo, "Miner Info successfully fetched"},
		{de.getPeerAddr, "Miner address validated"},
		{de.netConnect, "Connected to miner"},
		{de.netDiag, "Got miner version"},
	})
	cancel()
	if err != nil {
		return err
	}
	stage = "QueryAsk"
	stageCtx, cancel = getStageCtx(stage)
	err = executeStage(stageCtx, stage, updateStage, []step{
		{de.queryAsk, "Successfully queried ask from miner"},
	})
	cancel()
	if err != nil {
		return err
	}
	stage = "CheckPrice"
	stageCtx, cancel = getStageCtx(stage)
	err = executeStage(stageCtx, stage, updateStage, []step{
		{de.checkPrice, "Deal Price Validated"},
	})
	cancel()
	if err != nil {
		return err
	}
	stage = "ClientImport"
	stageCtx, cancel = getStageCtx(stage)
	err = executeStage(stageCtx, stage, updateStage, []step{
		{de.generateFile, "Data file generated"},
		{de.importFile, "Data file imported to Lotus"},
	})
	cancel()
	if err != nil {
		return err
	}
	return de.executeAndMonitorDeal(ctx, updateStage, stageTimeouts)
}

type dealExecutor struct {
	ctx           context.Context
	config        NodeConfig
	node          api.FullNode
	miner         string
	log           LogStatus
	tipSet        *types.TipSet
	minerAddress  address.Address
	minerInfo     miner.MinerInfo
	pi            peer.AddrInfo
	releaseWorker func()
	makeHost      func(ctx context.Context, opts ...config.Option) (host.Host, error)
}

type storageDealExecutor struct {
	dealExecutor
	task      StorageTask
	price     big.Int
	fileName  string
	importRes *api.ImportRes
}

func (de *dealExecutor) getTipSet(_ logFunc) (err error) {
	de.tipSet, err = de.node.ChainHead(de.ctx)
	return err
}

func (de *dealExecutor) getMinerInfo(_ logFunc) (err error) {
	// retrieve and validate miner price
	de.minerAddress, err = address.NewFromString(de.miner)
	if err != nil {
		return err
	}

	de.minerInfo, err = de.node.StateMinerInfo(de.ctx, de.minerAddress, de.tipSet.Key())
	return err
}

func (de *dealExecutor) getPeerAddr(_ logFunc) error {
	if de.minerInfo.PeerId == nil {
		return errors.New("no PeerID for miner")
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

func (de *dealExecutor) netConnect(_ logFunc) error {
	return de.node.NetConnect(de.ctx, de.pi)
}

func (de *dealExecutor) netDiag(l logFunc) error {
	av, err := de.node.NetAgentVersion(de.ctx, de.pi.ID)
	if err != nil {
		return err
	}
	de.log("remote peer version", "version", av)

	cv, err := de.node.Version(de.ctx)
	de.log("local client version", "version", cv.Version)

	l(fmt.Sprintf("NetAgentVersion: %s", av))
	l(fmt.Sprintf("ClientVersion: %s", cv.Version))
	if err != nil {
		return err
	}

	peerAddr, peerL, err := netutil.TryAcquireLatency(de.ctx, de.pi, de.makeHost)
	if err == nil {
		l(fmt.Sprintf("RemotePeerAddr: %s", peerAddr))
		l(fmt.Sprintf("RemotePeerLatency: %d", peerL))
	}
	return nil
}

func (de *storageDealExecutor) queryAsk(_ logFunc) (err error) {
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

func (de *storageDealExecutor) checkPrice(_ logFunc) error {
	maxPrice := abi.NewTokenAmount(de.task.MaxPriceAttoFIL.x)
	if de.task.MaxPriceAttoFIL.x == 0 {
		maxPrice = abi.NewTokenAmount(maxPriceDefault)
	}
	if de.price.GreaterThan(maxPrice) {
		return fmt.Errorf("miner ask price (%v) exceeds max price (%v)", de.price, maxPrice)
	}
	return nil
}

func (de *storageDealExecutor) generateFile(_ logFunc) error {
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

func (de *storageDealExecutor) importFile(l logFunc) (err error) {
	// import the file into the lotus node
	ref := api.FileRef{
		Path:  filepath.Join(de.config.NodeDataDir, de.fileName),
		IsCAR: false,
	}

	de.importRes, err = de.node.ClientImport(de.ctx, ref)
	if err != nil {
		return fmt.Errorf("error importing file: %w", err)
	}
	l(fmt.Sprintf("PayloadCID: %s", de.importRes.Root))
	return nil
}

func (de *storageDealExecutor) executeAndMonitorDeal(ctx context.Context, updateStage UpdateStage, stageTimeouts map[string]time.Duration) error {

	stage := "ProposeDeal"
	dealStage := CommonStages[stage]()
	err := updateStage(ctx, stage, dealStage)
	if err != nil {
		return err
	}

	startOffset := de.task.StartOffset.x
	if startOffset == 0 {
		startOffset = startOffsetDefault
	}

	de.log("imported deal file, got data cid", "datacid", de.importRes.Root)

	// price is in fil/gib/epoch so total EpochPrice is price * deal size / 1GB
	gib := big.NewInt(1 << 30)
	dealSize, err := de.node.ClientDealSize(ctx, de.importRes.Root)
	if err != nil {
		return err
	}
	epochPrice := big.Div(big.Mul(de.price, big.NewInt(int64(dealSize.PieceSize))), gib)

	// Prepare parameters for deal
	params := &api.StartDealParams{
		Data: &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         de.importRes.Root,
		},
		Wallet:            de.config.WalletAddress,
		Miner:             de.minerAddress,
		EpochPrice:        epochPrice,
		MinBlocksDuration: 2880 * 180,
		DealStartEpoch:    de.tipSet.Height() + abi.ChainEpoch(startOffset),
		FastRetrieval:     de.task.FastRetrieval.x,
		VerifiedDeal:      de.task.Verified.x,
	}
	_ = params

	// track updates to all deals -- we want to start this before we even
	// initiate the deal so we don't miss any updates
	updates, err := de.node.ClientGetDealUpdates(de.ctx)
	if err != nil {
		return err
	}

	de.log("got deal updates channel")

	// start deal process
	proposalCid, err := de.node.ClientStartDeal(de.ctx, params)
	if err != nil {
		return err
	}

	de.log("got proposal cid", "cid", proposalCid)
	dealStage = AddLog(dealStage, fmt.Sprintf("ProposalCID: %s", proposalCid))
	if err := updateStage(ctx, stage, dealStage); err != nil {
		return err
	}

	var prevStage string
	var loggedID bool = false
	defaultStageTimeout := stageTimeouts[defaultStorageStageTimeoutName]
	timer := time.NewTimer(defaultStageTimeout)

	lastState := storagemarket.StorageDealUnknown
	for {
		if stage != prevStage {
			// Set the timeout for the current deal stage
			timeout, ok := stageTimeouts[strings.ToLower(stage)]
			if !ok {
				timeout = defaultStageTimeout
			}
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(timeout)
			prevStage = stage
		}

		select {
		case <-timer.C:
			timeout, ok := stageTimeouts[strings.ToLower(stage)]
			if !ok {
				timeout = defaultStageTimeout
			}
			msg := fmt.Sprintf("timed out after %s", timeout)
			dealStage = AddLog(dealStage, msg)
			return fmt.Errorf("deal stage %q %s", stage, msg)
		case info, ok := <-updates:
			if !ok {
				return fmt.Errorf("Client error: Update channel closed before deal is active")
			}

			if !proposalCid.Equals(info.ProposalCid) {
				continue
			}

			if info.State != lastState {
				de.log("Deal status",
					"cid", info.ProposalCid,
					"piece", info.PieceCID,
					"state", storagemarket.DealStates[info.State],
					"deal_message", info.Message,
					"provider", info.Provider,
				)
				lastState = info.State
				if info.State == storagemarket.StorageDealCheckForAcceptance {
					de.releaseWorker()
				}
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
				return fmt.Errorf("storage deal failed: %s", info.Message)
			}

			if len(info.DealStages.Stages) > 0 {
				newState := storagemarket.DealStates[info.State]
				newDetails := toStageDetails(info.DealStages.Stages[len(info.DealStages.Stages)-1])

				if !loggedID && info.DealID != 0 {
					loggedID = true
					newDetails = AddLog(newDetails, fmt.Sprintf("DealID: %d", info.DealID))
				}
				err = updateStage(ctx, newState, newDetails)

				if err != nil {
					return err
				}

				stage = newState
			}

			// deal is on chain, exit successfully
			if info.State == storagemarket.StorageDealActive {
				logStages(info, de.log)
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (de *storageDealExecutor) cleanupDeal() error {
	if de.importRes != nil {
		// clear out the lotus import store for CIDs in this deal
		err := de.node.ClientRemoveImport(de.ctx, de.importRes.ImportID)
		if err != nil {
			return err
		}
		de.importRes = nil
	}
	if de.fileName != "" {
		// as a last step no matter what ended up happening with the deal, delete the generated file off to reclaim disk space
		err := os.Remove(filepath.Join(de.config.DataDir, de.fileName))
		if err != nil {
			return err
		}
		de.fileName = ""
	}
	return nil
}

func mktime(t time.Time) _Time {
	return _Time{x: t.UnixNano()}
}

func asStrM(s string) _String__Maybe {
	return _String__Maybe{m: schema.Maybe_Value, v: &_String{s}}
}
func asIntM(i int64) _Int__Maybe {
	return _Int__Maybe{m: schema.Maybe_Value, v: &_Int{i}}
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
			"deal_message", info.Message,
			"provider", info.Provider,
			"price", info.PricePerEpoch,
			"verfied", info.Verified,
		)
	}
}

func (sp _StorageTask__Prototype) Of(miner string, maxPrice, size, startOffset int64, fastRetrieval, verified bool, tag string) StorageTask {
	st := _StorageTask{
		Miner:           _String{miner},
		MaxPriceAttoFIL: _Int{maxPrice},
		Size:            _Int{size},
		StartOffset:     _Int{startOffset},
		FastRetrieval:   _Bool{fastRetrieval},
		Verified:        _Bool{verified},
		Tag:             asStrM(tag),
	}
	return &st
}
