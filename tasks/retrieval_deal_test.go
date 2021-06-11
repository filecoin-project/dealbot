package tasks

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/mocks"
	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multicodec"
	"github.com/stretchr/testify/require"
)

func TestCancelOldDeals(t *testing.T) {
	ctx := context.Background()
	other := generateRandomPeer()
	root := generateRandomCID(t)
	cancelID1 := retrievalmarket.DealID(rand.Uint64())
	cancelID2 := retrievalmarket.DealID(rand.Uint64())

	testCases := map[string]struct {
		setExpectations func(t *testing.T, expect *mocks.MockFullNodeMockRecorder)
		expectedErr     error
		expectedLogs    []string
	}{
		"returns deals but non cancelled": {
			setExpectations: func(t *testing.T, expect *mocks.MockFullNodeMockRecorder) {
				expect.ClientListRetrievals(gomock.Any()).Return(
					[]api.RetrievalInfo{
						{
							ID:         retrievalmarket.DealID(rand.Uint64()),
							PayloadCID: generateRandomCID(t),
							Provider:   generateRandomPeer(),
						},
						{
							ID:         retrievalmarket.DealID(rand.Uint64()),
							PayloadCID: generateRandomCID(t),
							Provider:   generateRandomPeer(),
						},
					}, nil,
				)
			},
		},
		"returns error when fetching retrievals": {
			setExpectations: func(t *testing.T, expect *mocks.MockFullNodeMockRecorder) {
				expect.ClientListRetrievals(gomock.Any()).Return(
					[]api.RetrievalInfo{}, errors.New("something went wrong"),
				)
			},
			expectedErr: errors.New("something went wrong"),
		},
		"cancels a retrieval": {
			setExpectations: func(t *testing.T, expect *mocks.MockFullNodeMockRecorder) {
				expect.ClientListRetrievals(gomock.Any()).Return(
					[]api.RetrievalInfo{
						{
							ID:         retrievalmarket.DealID(rand.Uint64()),
							PayloadCID: generateRandomCID(t),
							Provider:   generateRandomPeer(),
						},
						{
							ID:         cancelID1,
							PayloadCID: root,
							Provider:   other,
						},
					}, nil,
				)
				expect.ClientCancelRetrievalDeal(gomock.Any(), gomock.Eq(cancelID1)).Return(nil)
			},
			expectedLogs: []string{
				fmt.Sprintf("cancelled identical retrieval with ID %d", cancelID1),
			},
		},
		"retrieval cancel errors": {
			setExpectations: func(t *testing.T, expect *mocks.MockFullNodeMockRecorder) {
				expect.ClientListRetrievals(gomock.Any()).Return(
					[]api.RetrievalInfo{
						{
							ID:         retrievalmarket.DealID(rand.Uint64()),
							PayloadCID: generateRandomCID(t),
							Provider:   generateRandomPeer(),
						},
						{
							ID:         cancelID1,
							PayloadCID: root,
							Provider:   other,
						},
					}, nil,
				)
				expect.ClientCancelRetrievalDeal(gomock.Any(), gomock.Eq(cancelID1)).Return(errors.New("something went wrong"))
			},
			expectedErr: errors.New("something went wrong"),
		},
		"cancels multiple retrievals": {
			setExpectations: func(t *testing.T, expect *mocks.MockFullNodeMockRecorder) {
				expect.ClientListRetrievals(gomock.Any()).Return(
					[]api.RetrievalInfo{
						{
							ID:         cancelID1,
							PayloadCID: root,
							Provider:   other,
						},
						{
							ID:         cancelID2,
							PayloadCID: root,
							Provider:   other,
						},
					}, nil,
				)
				expect.ClientCancelRetrievalDeal(gomock.Any(), gomock.Eq(cancelID1)).Return(nil)
				expect.ClientCancelRetrievalDeal(gomock.Any(), gomock.Eq(cancelID2)).Return(nil)
			},
			expectedLogs: []string{
				fmt.Sprintf("cancelled identical retrieval with ID %d", cancelID1),
				fmt.Sprintf("cancelled identical retrieval with ID %d", cancelID2),
			},
		},
	}

	for testCase, data := range testCases {
		t.Run(testCase, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			dealStage := CommonStages["ProposeDeal"]()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			node := mocks.NewMockFullNode(ctrl)
			de := &retrievalDealExecutor{
				dealExecutor: dealExecutor{
					ctx: ctx,
					pi: peer.AddrInfo{
						ID: other,
					},
					node: node,
					log:  func(msg string, keysAndValues ...interface{}) {},
				},
				task: &_RetrievalTask{PayloadCID: _String{root.String()}},
				offer: api.QueryOffer{
					Root: root,
				},
			}
			data.setExpectations(t, node.EXPECT())
			dealStage, err := de.cancelOldDeals(ctx, dealStage)
			if data.expectedErr != nil {
				require.EqualError(t, err, data.expectedErr.Error())
			} else {
				require.NoError(t, err)
				for _, expectedLog := range data.expectedLogs {
					assertHasLogWithPrefix(t, dealStage.FieldLogs(), expectedLog)
				}
			}
		})
	}
}

func generateRandomCID(t *testing.T) cid.Cid {
	prefix := cid.Prefix{
		Version:  1,
		Codec:    uint64(multicodec.Raw),
		MhType:   uint64(multicodec.Sha2_256),
		MhLength: 32,
	}
	buf := make([]byte, 100)
	_, _ = rand.Read(buf)
	c, err := prefix.Sum(buf)
	require.NoError(t, err)
	return c
}

func generateRandomPeer() peer.ID {
	buf := make([]byte, 100)
	_, _ = rand.Read(buf)
	return peer.ID(buf)
}
