package lotus

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
)

func NewAPIWrapper(node api.FullNode, store adt.Store) *APIWrapper {
	return &APIWrapper{
		FullNode: node,
		store:    store,
	}
}

type APIWrapper struct {
	api.FullNode
	store adt.Store
}

func (aw *APIWrapper) Store() adt.Store {
	return aw.store
}

func (aw *APIWrapper) StartDeal(ctx context.Context, params *api.StartDealParams) (*cid.Cid, error) {
	return aw.FullNode.ClientStartDeal(ctx, params)
}

func (aw *APIWrapper) Import(ctx context.Context, ref api.FileRef) (*api.ImportRes, error) {
	return aw.FullNode.ClientImport(ctx, ref)
}

func (aw *APIWrapper) QueryAsk(ctx context.Context, p peer.ID, miner address.Address) (*storagemarket.StorageAsk, error) {
	return aw.FullNode.ClientQueryAsk(ctx, p, miner)
}

func (aw *APIWrapper) ChainHead(ctx context.Context) (*types.TipSet, error) {
	return aw.FullNode.ChainHead(ctx)
}

func (aw *APIWrapper) MinerInfo(ctx context.Context, a address.Address, tsk types.TipSetKey) (miner.MinerInfo, error) {
	return aw.FullNode.StateMinerInfo(ctx, a, tsk)
}

func (aw *APIWrapper) DealPieceCID(ctx context.Context, root cid.Cid) (api.DataCIDSize, error) {
	return aw.FullNode.ClientDealPieceCID(ctx, root)
}

func (aw *APIWrapper) GetDealUpdates(ctx context.Context) (<-chan api.DealInfo, error) {
	return aw.FullNode.ClientGetDealUpdates(ctx)
}
