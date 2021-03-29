package lotus

import (
	"context"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
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


func (aw *APIWrapper) MakeDeal(ctx context.Context, params *api.StartDealParams) (*cid.Cid, error) {
	return aw.FullNode.ClientStartDeal(ctx, params)
}


func (aw *APIWrapper) Import(ctx context.Context, ref api.FileRef) (*api.ImportRes, error) {
	return aw.FullNode.ClientImport(ctx, ref)
}