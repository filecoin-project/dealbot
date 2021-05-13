package tasks

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/lotus/api"
	"github.com/ipfs/go-cid"
)

func MakeRetrievalDeal(ctx context.Context, config NodeConfig, node api.FullNode, task RetrievalTask, updateStage UpdateStage, log LogStatus) error {
	de := &retrievalDealExecutor{
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
		{de.queryOffer, "Miner Offer Received"},
	})
	if err != nil {
		return err
	}
	return de.executeAndMonitorDeal(updateStage)
}

type retrievalDealExecutor struct {
	dealExecutor
	task  RetrievalTask
	offer api.QueryOffer
}

func (de *retrievalDealExecutor) queryOffer() error {
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
	return nil
}

func (de *retrievalDealExecutor) executeAndMonitorDeal(updateStage UpdateStage) error {
	dealStage := RetrievalStages["ProposeRetrieval"]
	err := updateStage("ProposeRetrieval", dealStage)
	if err != nil {
		return err
	}

	ref := &api.FileRef{
		Path:  filepath.Join(de.config.NodeDataDir, "ret"),
		IsCAR: de.task.CARExport.x,
	}

	events, err := de.node.ClientRetrieveWithEvents(de.ctx, de.offer.Order(de.config.WalletAddress), ref)
	if err != nil {
		return err
	}

	dealStage = AddLog(dealStage, "deal sent to miner")
	err = updateStage("ProposeRetrieval", dealStage)
	if err != nil {
		return err
	}

	lastStatus := retrievalmarket.DealStatusNew
	var lastBytesReceived uint64 = 0
	for event := range events {
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
			dealStage = RetrievalStages["DealAccepted"]
			err := updateStage("DealAccepted", dealStage)
			if err != nil {
				return err
			}
		}
		if event.BytesReceived > 0 && lastBytesReceived == 0 {
			dealStage = RetrievalStages["FirstByteReceived"]
			err := updateStage("FirstByteReceived", dealStage)
			if err != nil {
				return err
			}
		}
		switch event.Status {
		case retrievalmarket.DealStatusCancelled,
			retrievalmarket.DealStatusErrored,
			retrievalmarket.DealStatusRejected,
			retrievalmarket.DealStatusDealNotFound:

			return errors.New("retrieval deal failed")

		// deal is on chain, exit successfully
		case retrievalmarket.DealStatusCompleted:
			dealStage = RetrievalStages["DealComplete"]
			dealStage = AddLog(dealStage, fmt.Sprintf("bytes received: %d", event.BytesReceived))
			err := updateStage("DealComplete", dealStage)
			if err != nil {
				return err
			}

			rdata, err := ioutil.ReadFile(filepath.Join(de.config.DataDir, "ret"))
			if err != nil {
				return err
			}

			de.log("retrieval successful", "PayloadCID", de.task.PayloadCID.x)

			_ = rdata

			dealStage = AddLog(dealStage, "file read from file system")
			err = updateStage("DealComplete", dealStage)
			if err != nil {
				return err
			}

			if de.task.CARExport.x {
				return errors.New("car export not implemented")
			}
			return nil
		}
	}

	return nil
}

func (rp *_RetrievalTask__Prototype) Of(minerParam string, payloadCid string, carExport bool) RetrievalTask {
	rt := _RetrievalTask{
		Miner:      _String{minerParam},
		PayloadCID: _String{payloadCid},
		CARExport:  _Bool{carExport},
	}
	return &rt
}
