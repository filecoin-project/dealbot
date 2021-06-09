package tasks

import (
	"context"
	"strings"
	"testing"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/mocks"
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
