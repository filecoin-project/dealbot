package tasks

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/lotus/api"
	"github.com/ipfs/go-cid"
)

func MakeRetrievalDeal(ctx context.Context, config NodeConfig, node api.FullNode, task RetrievalTask, updateStage UpdateStage, log LogStatus, stageTimeouts map[string]time.Duration) error {
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

	return de.executeAndMonitorDeal(ctx, updateStage, stageTimeouts)
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

func (de *retrievalDealExecutor) executeAndMonitorDeal(ctx context.Context, updateStage UpdateStage, stageTimeouts map[string]time.Duration) error {
	stage := "ProposeDeal"
	dealStage := CommonStages[stage]
	err := updateStage(stage, dealStage)
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
	err = updateStage(stage, dealStage)
	if err != nil {
		return err
	}

	lastStatus := retrievalmarket.DealStatusNew
	var lastBytesReceived uint64
	var prevStage string

	stageTimer := time.NewTimer(0)
	if !stageTimer.Stop() {
		<-stageTimer.C
	}
	defer stageTimer.Stop()

	for {
		if stage != prevStage {
			// If there is a timeout for the current deal stage, then set the timer
			if stageTimeout, ok := stageTimeouts[strings.ToLower(stage)]; ok {
				stageTimer.Reset(stageTimeout)
			} else {
				if !stageTimer.Stop() {
					<-stageTimer.C
				}
			}
			prevStage = stage
		}

		select {
		case <-stageTimer.C:
			msg := fmt.Sprintf("timed out after %s", stageTimeouts[strings.ToLower(stage)])
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
					stage = "DealAccepted"
					dealStage = RetrievalStages[stage]
					err := updateStage(stage, dealStage)
					if err != nil {
						return err
					}
				}
				if event.BytesReceived > 0 && lastBytesReceived == 0 {
					stage = "FirstByteReceived"
					dealStage = RetrievalStages[stage]
					err := updateStage(stage, dealStage)
					if err != nil {
						return err
					}
				}
			} else {
				// steam closed, no errors, so the deal is a success

				// deal is on chain, exit successfully
				stage = "DealComplete"
				dealStage = RetrievalStages[stage]
				dealStage = AddLog(dealStage, fmt.Sprintf("bytes received: %d", event.BytesReceived))
				err := updateStage(stage, dealStage)
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
				err = updateStage(stage, dealStage)
				if err != nil {
					return err
				}

				if de.task.CARExport.x {
					return errors.New("car export not implemented")
				}
				return nil
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

func (rp *_RetrievalTask__Prototype) Of(minerParam string, payloadCid string, carExport bool, tag string) RetrievalTask {
	rt := _RetrievalTask{
		Miner:      _String{minerParam},
		PayloadCID: _String{payloadCid},
		CARExport:  _Bool{carExport},
		Tag:        asStrM(tag),
	}
	return &rt
}

// ParseStageTimeouts parses "StageName=timeout" strings into maps of retrieval and
// storage timeouts.
func ParseStageTimeouts(timeoutSpecs []string) (map[string]time.Duration, map[string]time.Duration, error) {
	var (
		retrievalTimeouts map[string]time.Duration
		storageTimeouts   map[string]time.Duration
	)

	// Parse all stage timeout durations
	stageTimeouts := map[string]time.Duration{}
	for _, spec := range timeoutSpecs {
		parts := strings.SplitN(spec, "=", 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid stage timeout specification: %s", spec)
		}
		stage := parts[0]
		timeout := parts[1]
		d, err := time.ParseDuration(strings.TrimSpace(timeout))
		if err != nil || d < time.Second {
			return nil, nil, fmt.Errorf("invalid value for stage %q timeout: %s", stage, timeout)
		}
		stageTimeouts[strings.ToLower(strings.TrimSpace(stage))] = d
	}

	// Get retrieval timeouts
	for stageName, _ := range RetrievalStages {
		stageName = strings.ToLower(stageName)
		d, ok := stageTimeouts[stageName]
		if ok {
			if retrievalTimeouts == nil {
				retrievalTimeouts = make(map[string]time.Duration)
			}
			retrievalTimeouts[stageName] = d
		}
	}

	// Get storage timeouts
	//
	// None defined currently

	return retrievalTimeouts, storageTimeouts, nil
}
