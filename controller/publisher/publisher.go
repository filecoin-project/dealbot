package publisher

import (
	"context"

	"github.com/ipfs/go-cid"
)

type Publisher interface {
	Start(context.Context) error
	Publish(context.Context, cid.Cid) error
	Shutdown(context.Context) error
}
