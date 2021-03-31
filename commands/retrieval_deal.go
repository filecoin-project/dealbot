package commands

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var MakeRetrievalDeal = &cli.Command{
	Name:  "retrieval-deal",
	Usage: "Make retrieval deals with provided miners.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "cid",
			Usage: "payload cid to fetch from miner",
			Value: "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
		},
		&cli.StringFlag{
			Name:  "miner",
			Usage: "miner to fetch content from",
			Value: "f0127896",
		},
	},
	Action: makeRetrievalDeal,
}

func makeRetrievalDeal(cctx *cli.Context) error {
	if err := setupLogging(cctx); err != nil {
		return xerrors.Errorf("setup logging: %w", err)
	}

	// start API to lotus node
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

	v, err := node.Version(context.Background())
	if err != nil {
		return err
	}

	log.Infof("remote version: %s", v.Version)

	carExport := true
	payloadCid, err := cid.Parse(cctx.String("cid"))
	if err != nil {
		panic(err)
	}

	log.Infof("retrieving cid: %s", payloadCid)

	err = RetrieveData(context.Background(), node, cctx.String("miner"), payloadCid, carExport)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("successfully retrieved")

	return nil
}

func RetrieveData(ctx context.Context, client lotus.API, miner string, fcid cid.Cid, carExport bool) error {
	minerAddr, err := address.NewFromString(miner)
	if err != nil {
		return err
	}

	offer, err := client.ClientMinerQueryOffer(ctx, minerAddr, fcid, nil)
	if err != nil {
		return err
	}

	log.Info("got offer")

	rpath, err := ioutil.TempDir("", "dealbot-retrieve-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(rpath)

	caddr, err := client.WalletDefaultAddress(ctx)
	if err != nil {
		return err
	}

	log.Info("got wallet")

	ref := &api.FileRef{
		Path:  filepath.Join(rpath, "ret"),
		IsCAR: carExport,
	}

	err = client.ClientRetrieve(ctx, offer.Order(caddr), ref)
	if err != nil {
		return err
	}

	rdata, err := ioutil.ReadFile(filepath.Join(rpath, "ret"))
	if err != nil {
		return err
	}

	_ = rdata

	//if carExport {
	//rdata = ExtractCarData(ctx, rdata, rpath)
	//}

	return nil
}
