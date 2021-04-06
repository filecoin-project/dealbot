package tasks

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/ipfs/go-cid"
)

type RetrievalTask struct {
	Miner      string
	PayloadCID string
	CarExport  bool
}

func (t *RetrievalTask) FromMap(m map[string]interface{}) error {
	if ms, ok := m["Miner"]; ok {
		if s, ok := ms.(string); ok {
			t.Miner = s
		}
	} else {
		return fmt.Errorf("retrieval task JSON missing `Miner` field: %v", m)
	}

	if ps, ok := m["PayloadCID"]; ok {
		if s, ok := ps.(string); ok {
			t.PayloadCID = s
		}
	} else {
		return fmt.Errorf("retrieval task JSON missing `PayloadCID` field: %v", m)
	}

	if cs, ok := m["CarExport"]; ok {
		if b, ok := cs.(bool); ok {
			t.CarExport = b
		}
	}

	return nil
}

func MakeRetrievalDeal(ctx context.Context, config ClientConfig, node api.FullNode, task RetrievalTask, log UpdateStatus) error {
	payloadCid, err := cid.Parse(task.PayloadCID)
	if err != nil {
		return err
	}

	minerAddr, err := address.NewFromString(task.Miner)
	if err != nil {
		return err
	}

	offer, err := node.ClientMinerQueryOffer(ctx, minerAddr, payloadCid, nil)
	if err != nil {
		return err
	}

	if offer.Err != "" {
		return fmt.Errorf("got error in offer: %s", offer.Err)
	}

	log("got query offer", "root", offer.Root, "piece", offer.Piece, "size", offer.Size, "minprice", offer.MinPrice, "unseal_price", offer.UnsealPrice)

	ref := &api.FileRef{
		Path:  filepath.Join(config.NodeDataDir, "ret"),
		IsCAR: task.CarExport,
	}

	err = node.ClientRetrieve(ctx, offer.Order(config.WalletAddress), ref)
	if err != nil {
		return err
	}

	rdata, err := ioutil.ReadFile(filepath.Join(config.DataDir, "ret"))
	if err != nil {
		return err
	}

	log("retrieval successful", "PayloadCID", task.PayloadCID)

	_ = rdata

	//if carExport {
	//rdata = ExtractCarData(ctx, rdata, rpath)
	//}

	return nil
}
