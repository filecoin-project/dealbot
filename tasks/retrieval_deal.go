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
	Miner      string `json:"miner"`
	PayloadCID string `json:"payload_cid"`
	CARExport  bool   `json:"car_export"`
}

func (t *RetrievalTask) FromMap(m map[string]interface{}) error {
	if ms, ok := m["miner"]; ok {
		if s, ok := ms.(string); ok {
			t.Miner = s
		}
	} else {
		return fmt.Errorf("retrieval task JSON missing `miner` field: %v", m)
	}

	if ps, ok := m["payload_cid"]; ok {
		if s, ok := ps.(string); ok {
			t.PayloadCID = s
		}
	} else {
		return fmt.Errorf("retrieval task JSON missing `payload_cid` field: %v", m)
	}

	if cs, ok := m["car_export"]; ok {
		if b, ok := cs.(bool); ok {
			t.CARExport = b
		}
	}

	return nil
}

func MakeRetrievalDeal(ctx context.Context, config NodeConfig, node api.FullNode, task RetrievalTask, log UpdateStatus) error {
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
		IsCAR: task.CARExport,
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
