package tasks

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/mocks"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/config"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestNetDiag(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mn := mocknet.New(ctx)
	other, err := mn.GenPeer()
	require.NoError(t, err)
	node := mocks.NewMockFullNode(ctrl)
	node.EXPECT().NetAgentVersion(gomock.Eq(ctx), gomock.Eq(other.ID())).Return("1.12.0", nil)
	node.EXPECT().Version(gomock.Eq(ctx)).Return(api.APIVersion{
		Version: "1.11.0",
	}, nil)
	de := &dealExecutor{
		ctx: ctx,
		pi: peer.AddrInfo{
			ID:    other.ID(),
			Addrs: other.Addrs(),
		},
		node: node,
		makeHost: func(ctx context.Context, _ ...config.Option) (host.Host, error) {
			h, err := mn.GenPeer()
			if err == nil {
				mn.LinkAll()
			}
			return h, err
		},
		log: func(msg string, keysAndValues ...interface{}) {},
	}
	var finalStageDetails StageDetails
	executeStage(ctx, "MinerOnline", func(ctx context.Context, stage string, stageDetails StageDetails) error {
		finalStageDetails = stageDetails
		return nil
	}, []step{
		{
			stepExecution: de.netDiag,
			stepSuccess:   "Got Miner Version",
		},
	})
	minerVersion := assertHasLogWithPrefix(t, finalStageDetails.FieldLogs(), "NetAgentVersion: ")
	require.Equal(t, "1.12.0", minerVersion)
	clientVersion := assertHasLogWithPrefix(t, finalStageDetails.FieldLogs(), "ClientVersion: ")
	require.Equal(t, "1.11.0", clientVersion)
	remotePeerAddr := assertHasLogWithPrefix(t, finalStageDetails.FieldLogs(), "RemotePeerAddr: ")
	require.Equal(t, other.Addrs()[0].String(), remotePeerAddr)
	_ = assertHasLogWithPrefix(t, finalStageDetails.FieldLogs(), "RemotePeerLatency: ")
}

func TestExecuteDeal(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	node := mocks.NewMockFullNode(ctrl)
	root := generateRandomCID(t)
	proposalCid := generateRandomCID(t)
	basePrice := abi.NewTokenAmount(1000000000000)
	dealInfo := make(chan api.DealInfo, 1)
	dealInfo <- api.DealInfo{
		ProposalCid: proposalCid,
		State:       storagemarket.StorageDealActive,
		DealStages:  &storagemarket.DealStages{},
	}

	de := &storageDealExecutor{
		dealExecutor: dealExecutor{
			ctx:    ctx,
			node:   node,
			log:    func(msg string, keysAndValues ...interface{}) {},
			tipSet: &types.TipSet{},
		},
		task: Type.StorageTask.Of("f1000", 1000000000000, 21474836480, 6152, true, true, "verified"),
		importRes: &api.ImportRes{
			Root: root,
		},
		price: basePrice,
	}
	node.EXPECT().ClientDealSize(gomock.Eq(ctx), gomock.Eq(root)).Return(api.DataSize{
		// 32GB Deal
		PieceSize: 1 << 35,
	}, nil)

	node.EXPECT().ClientStartDeal(gomock.Eq(ctx), gomock.Eq(&api.StartDealParams{
		Data: &storagemarket.DataRef{
			TransferType: storagemarket.TTGraphsync,
			Root:         de.importRes.Root,
		},
		Wallet:            de.config.WalletAddress,
		Miner:             de.minerAddress,
		EpochPrice:        big.Mul(basePrice, big.NewInt(32)),
		MinBlocksDuration: 2880 * 180,
		DealStartEpoch:    de.tipSet.Height() + abi.ChainEpoch(6152),
		FastRetrieval:     de.task.FastRetrieval.x,
		VerifiedDeal:      de.task.Verified.x,
	})).Return(&proposalCid, nil)

	node.EXPECT().ClientGetDealUpdates(gomock.Eq(ctx)).Return(dealInfo, nil)

	err := de.executeAndMonitorDeal(ctx, func(ctx context.Context, stage string, stageDetails StageDetails) error { return nil }, map[string]time.Duration{})
	require.NoError(t, err)
}

func assertHasLogWithPrefix(t *testing.T, logs List_Logs, prefix string) (postfix string) {
	itr := logs.Iterator()
	for !itr.Done() {
		_, log := itr.Next()
		if strings.Contains(log.FieldLog().String(), prefix) {
			return strings.TrimPrefix(log.FieldLog().String(), prefix)
		}
	}
	t.Fatalf("expected logs to contain a string that begins with: '%s'", prefix)
	return ""
}
