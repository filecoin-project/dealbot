package tasks

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/clientstates"
	"github.com/filecoin-project/lotus/api"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	"github.com/libp2p/go-libp2p"
	"golang.org/x/xerrors"
)

type RetrievalStage = string

const (
	RetrievalStageProposeDeal       = RetrievalStage("ProposeDeal")
	RetrievalStageDealAccepted      = RetrievalStage("DealAccepted")
	RetrievalStageFirstByteReceived = RetrievalStage("FirstByteReceived")
	RetrievalStageAllBytesReceived  = RetrievalStage("AllBytesReceived")
	RetrievalStageDealComplete      = RetrievalStage("DealComplete")
)

var RetrievalDealStages = []RetrievalStage{
	RetrievalStageProposeDeal,
	RetrievalStageDealAccepted,
	RetrievalStageFirstByteReceived,
	RetrievalStageAllBytesReceived,
	RetrievalStageDealComplete,
}

func MakeRetrievalDeal(ctx context.Context, config NodeConfig, node api.FullNode, task RetrievalTask, updateStage UpdateStage, log LogStatus, stageTimeouts map[string]time.Duration, releaseWorker func()) error {
	de := &retrievalDealExecutor{
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

	defaultTimeout := stageTimeouts[defaultRetrievalStageTimeoutName]
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
		{de.queryOffer, "Miner Offer Received"},
	})
	cancel()
	if err != nil {
		return err
	}

	err = de.executeAndMonitorDeal(ctx, updateStage, stageTimeouts)
	cleanupErr := de.cleanupDeal()
	if err != nil {
		return err
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	stage = RetrievalStageDealComplete
	dealStage := RetrievalStages[stage]()
	return updateStage(ctx, stage, dealStage)
}

type retrievalDealExecutor struct {
	dealExecutor
	task  RetrievalTask
	offer api.RetrievalOrder
}

func (de *retrievalDealExecutor) queryOffer(logg logFunc) error {
	payloadCid, err := cid.Parse(de.task.PayloadCID.x)
	if err != nil {
		return err
	}

	qo, err := de.node.ClientMinerQueryOffer(de.ctx, de.minerAddress, payloadCid, nil)
	if err != nil {
		return err
	}
	de.offer = qo.Order(de.config.WalletAddress)

	if qo.Err != "" {
		return fmt.Errorf("got error in offer: %s", qo.Err)
	}

	de.log("got query offer", "root", de.offer.Root, "piece", de.offer.Piece, "size", de.offer.Size, "minprice", qo.MinPrice, "unseal_price", de.offer.UnsealPrice)

	if de.task.MaxPriceAttoFIL.Exists() {
		sizePrice := big.NewInt(2).Mul(qo.MinPrice.Int, big.NewInt(0).SetUint64(de.offer.Size))
		totPrice := big.NewInt(0).Add(de.offer.UnsealPrice.Int, sizePrice)
		if totPrice.Cmp(big.NewInt(de.task.MaxPriceAttoFIL.Must().Int())) == 1 {
			// too expensive.
			msg := fmt.Sprintf("RejectingDealOverCost min:%d, unseal:%d", qo.MinPrice.Int64(), de.offer.UnsealPrice.Int64())
			logg(msg)
			return fmt.Errorf(msg)
		}
	}

	return nil
}

func (de *retrievalDealExecutor) cancelOldDeals(ctx context.Context, dealStage StageDetails) (StageDetails, error) {
	retrievals, err := de.node.ClientListRetrievals(ctx)
	if err != nil {
		return nil, err
	}
	for _, retrieval := range retrievals {
		if retrieval.Provider == de.pi.ID && retrieval.PayloadCID.Equals(de.offer.Root) && !clientstates.IsFinalityState(retrieval.Status) {
			err := de.node.ClientCancelRetrievalDeal(ctx, retrieval.ID)
			if err != nil {
				return nil, err
			}
			dealStage = AddLog(dealStage, fmt.Sprintf("cancelled identical retrieval with ID %d", retrieval.ID))
			de.log("cancelled identical retrieval", "ID", retrieval.ID, "PayloadCID", de.task.PayloadCID, "MinerID", de.task.Miner)
		}
	}
	return dealStage, nil
}

func (de *retrievalDealExecutor) executeAndMonitorDeal(ctx context.Context, updateStage UpdateStage, stageTimeouts map[string]time.Duration) error {
	stage := RetrievalStageProposeDeal
	dealStage := CommonStages[stage]()
	err := updateStage(ctx, stage, dealStage)
	if err != nil {
		return err
	}

	// Clear out old retrieval deals
	dealStage, err = de.cancelOldDeals(ctx, dealStage)
	if err != nil {
		return err
	}
	err = updateStage(ctx, stage, dealStage)
	if err != nil {
		return err
	}

	ref := &api.FileRef{
		Path:  filepath.Join(de.config.NodeDataDir, de.task.PayloadCID.x),
		IsCAR: de.task.CARExport.x,
	}

	subscribeEvents, err := de.node.ClientGetRetrievalUpdates(ctx)
	if err != nil {
		return xerrors.Errorf("error setting up retrieval updates: %w", err)
	}

	retrievalRes, err := de.node.ClientRetrieve(ctx, de.offer)
	if err != nil {
		return err
	}

	dealStage = AddLog(dealStage, "deal sent to miner")
	err = updateStage(ctx, stage, dealStage)
	if err != nil {
		return err
	}

	lastStatus := retrievalmarket.DealStatusNew
	var lastBytesReceived uint64
	var prevStage string

	defaultStageTimeout := stageTimeouts[defaultRetrievalStageTimeoutName]
	timer := time.NewTimer(defaultStageTimeout)

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
		var event api.RetrievalInfo
		select {
		case <-timer.C:

			timeout, ok := stageTimeouts[strings.ToLower(stage)]
			if !ok {
				timeout = defaultStageTimeout
			}
			msg := fmt.Sprintf("timed out after %s", timeout)
			AddLog(dealStage, msg)
			return fmt.Errorf("deal stage %q %s", stage, msg)

		case event = <-subscribeEvents:
			if event.ID != retrievalRes.DealID {
				// we can't check the deal ID ahead of time because:
				// 1. We need to subscribe before retrieving.
				// 2. We won't know the deal ID until after retrieving.
				continue
			}

			// non-terminal event, process
			if event.Status != lastStatus {
				de.log("Deal status",
					"cid", de.task.PayloadCID.x,
					"state", retrievalmarket.DealStatuses[event.Status],
					"error", event.Message,
					"received", event.BytesReceived,
				)
				lastStatus = event.Status
			}

			if event.Event != nil && *event.Event == retrievalmarket.ClientEventDealAccepted {
				stage = RetrievalStageDealAccepted
				dealStage = RetrievalStages[stage]()
				//there should be a deal now.
				allRetrievals, err := de.node.ClientListRetrievals(ctx)
				if err == nil {
					for _, maybeOurRetrieval := range allRetrievals {
						if maybeOurRetrieval.Provider == de.pi.ID && maybeOurRetrieval.PayloadCID.Equals(de.offer.Root) && !clientstates.IsFinalityState(maybeOurRetrieval.Status) {
							dealStage = AddLog(dealStage, fmt.Sprintf("DealID: %d", maybeOurRetrieval.ID))
							break
						}
					}
				}
				err = updateStage(ctx, stage, dealStage)
				if err != nil {
					return err
				}
			}
			if event.BytesReceived > 0 && lastBytesReceived == 0 {
				stage = RetrievalStageFirstByteReceived
				dealStage = RetrievalStages[stage]()
				err := updateStage(ctx, stage, dealStage)
				if err != nil {
					return err
				}
			}

			if event.Status == retrievalmarket.DealStatusCompleted {
				// deal is on chain, exit successfully
				stage = RetrievalStageAllBytesReceived
				dealStage = RetrievalStages[stage]()
				dealStage = AddLog(dealStage, fmt.Sprintf("bytes received: %d", event.BytesReceived))
				err = updateStage(ctx, stage, dealStage)
				if err != nil {
					return err
				}

				eref := &api.ExportRef{
					Root:   de.offer.Root,
					DealID: retrievalRes.DealID,
				}

				err = de.node.ClientExport(ctx, *eref, *ref)
				if err != nil {
					return nil
				}

				rdata, err := os.Stat(filepath.Join(de.config.DataDir, de.task.PayloadCID.x))
				if err != nil {
					return err
				}

				de.log("retrieval successful", "PayloadCID", de.task.PayloadCID.x)

				_ = rdata

				dealStage = AddLog(dealStage, "file read from file system")
				err = updateStage(ctx, stage, dealStage)
				if err != nil {
					return err
				}

				// See if car and estimate number of CIDs in the piece.
				fp, err := os.Open(filepath.Join(de.config.DataDir, de.task.PayloadCID.x))
				if err == nil {
					defer fp.Close()
					numCids, err := tryReadCar(fp)
					if err == nil {
						dealStage = AddLog(dealStage, fmt.Sprintf("CIDCount: %d", numCids))
					}
					dealStage = AddLog(dealStage, fmt.Sprintf("CARReadClean: %t", err == nil))
					err = updateStage(ctx, stage, dealStage)
					if err != nil {
						return err
					}
				}
				return nil
			}
			if event.Status == retrievalmarket.DealStatusRejected {
				return fmt.Errorf("retrieval deal failed: Retrieval Proposal Rejected: %s", event.Message)
			}
			if event.Status ==
				retrievalmarket.DealStatusDealNotFound || event.Status ==
				retrievalmarket.DealStatusErrored {
				return fmt.Errorf("retrieval deal failed: Retrieval Error: %s", event.Message)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

func tryReadCar(fp *os.File) (numCids int, err error) {
	defer func() {
		if r := recover(); r != nil {
			numCids = 0
			err = fmt.Errorf("invalid car")
		}
	}()

	cr, err := car.NewCarReader(fp)
	if err == nil {
		for err == nil {
			_, err = cr.Next()
			if err == io.EOF {
				err = nil
				break
			}
			numCids++
		}
	}
	return numCids, err
}

func (de *retrievalDealExecutor) cleanupDeal() error {
	// clean up the data.
	imports, err := de.node.ClientListImports(de.ctx)
	if err != nil {
		return err
	}
	for _, i := range imports {
		if i.Root != nil && i.Root.String() == de.task.PayloadCID.String() {
			if err := de.node.ClientRemoveImport(de.ctx, i.Key); err != nil {
				return err
			}
		}
	}
	dbPath := filepath.Join(de.config.DataDir, de.task.PayloadCID.x)
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		if err := os.RemoveAll(dbPath); err != nil {
			return err
		}
	}
	return nil
}

func (rp _RetrievalTask__Prototype) Of(minerParam string, payloadCid string, carExport bool, tag string) RetrievalTask {
	rt := _RetrievalTask{
		Miner:      _String{minerParam},
		PayloadCID: _String{payloadCid},
		CARExport:  _Bool{carExport},
		Tag:        asStrM(tag),
	}
	return &rt
}

func (rp _RetrievalTask__Prototype) OfSchedule(minerParam string, payloadCid string, carExport bool, tag string, schedule string, scheduleLimit string, maxPrice int64) RetrievalTask {
	rt := _RetrievalTask{
		Miner:           _String{minerParam},
		PayloadCID:      _String{payloadCid},
		CARExport:       _Bool{carExport},
		Tag:             asStrM(tag),
		Schedule:        asStrM(schedule),
		ScheduleLimit:   asStrM(scheduleLimit),
		MaxPriceAttoFIL: asIntM(maxPrice),
	}
	return &rt
}
