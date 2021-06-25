package tasks

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/impl/clientstates"
	"github.com/filecoin-project/lotus/api"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
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

	return de.executeAndMonitorDeal(ctx, updateStage, stageTimeouts)
}

type retrievalDealExecutor struct {
	dealExecutor
	task  RetrievalTask
	offer api.QueryOffer
}

func (de *retrievalDealExecutor) queryOffer(logg logFunc) error {
	payloadCid, err := cid.Parse(de.task.PayloadCID.x)
	if err != nil {
		return err
	}

	de.offer, err = de.node.ClientMinerQueryOffer(de.ctx, de.minerAddress, payloadCid, nil)
	if err != nil {
		return err
	}

	if de.offer.Err != "" {
		return fmt.Errorf("got error in offer: %s", de.offer.Err)
	}

	de.log("got query offer", "root", de.offer.Root, "piece", de.offer.Piece, "size", de.offer.Size, "minprice", de.offer.MinPrice, "unseal_price", de.offer.UnsealPrice)

	if de.task.MaxPriceAttoFIL.Exists() {
		sizePrice := big.NewInt(2).Mul(de.offer.MinPrice.Int, big.NewInt(0).SetUint64(de.offer.Size))
		totPrice := big.NewInt(0).Add(de.offer.UnsealPrice.Int, sizePrice)
		if totPrice.Cmp(big.NewInt(de.task.MaxPriceAttoFIL.Must().Int())) == 1 {
			// too expensive.
			msg := fmt.Sprintf("RejectingDealOverCost min:%d, unseal:%d", de.offer.MinPrice.Int64(), de.offer.UnsealPrice.Int64())
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

	events, err := de.node.ClientRetrieveWithEvents(de.ctx, de.offer.Order(de.config.WalletAddress), ref)
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

		select {
		case <-timer.C:
			timeout, ok := stageTimeouts[strings.ToLower(stage)]
			if !ok {
				timeout = defaultStageTimeout
			}
			msg := fmt.Sprintf("timed out after %s", timeout)
			AddLog(dealStage, msg)
			return fmt.Errorf("deal stage %q %s", stage, msg)
		case event, ok := <-events:
			if ok {
				// non-terminal event, process

				if event.Status != lastStatus {
					de.log("Deal status",
						"cid", de.task.PayloadCID.x,
						"state", retrievalmarket.DealStatuses[event.Status],
						"error", event.Err,
						"received", event.BytesReceived,
					)
					lastStatus = event.Status
				}

				if event.Event == retrievalmarket.ClientEventDealAccepted {
					stage = RetrievalStageDealAccepted
					dealStage = RetrievalStages[stage]()
					err := updateStage(ctx, stage, dealStage)
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
			} else {
				// steam closed, no errors, so the deal is a success

				// deal is on chain, exit successfully
				stage = RetrievalStageAllBytesReceived
				dealStage = RetrievalStages[stage]()
				dealStage = AddLog(dealStage, fmt.Sprintf("bytes received: %d", event.BytesReceived))
				err = updateStage(ctx, stage, dealStage)
				if err != nil {
					return err
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

				// clean up the data.
				imports, err := de.node.ClientListImports(de.ctx)
				if err != nil {
					return err
				}
				for _, i := range imports {
					if i.Root.String() == de.task.PayloadCID.String() {
						if err := de.node.ClientRemoveImport(de.ctx, i.Key); err != nil {
							return err
						}
					}
				}
				dbPath := filepath.Join(de.config.DataDir, de.task.PayloadCID.x)
				if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
					if err := os.Remove(dbPath); err != nil {
						return err
					}
				}

				// final stage
				stage = RetrievalStageDealComplete
				dealStage = RetrievalStages[stage]()
				return updateStage(ctx, stage, dealStage)
			}

			// if the event has an error message, then something went wrong and deal failed
			if event.Err != "" {
				return fmt.Errorf("retrieval deal failed: %s", event.Err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

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
