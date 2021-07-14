package lotus

import (
	"context"
	"fmt"
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	cliutil "github.com/filecoin-project/lotus/cli/util"
	"github.com/urfave/cli/v2"
)

type GatewayOpener struct {
	addr    string
	headers http.Header
}

func (o *GatewayOpener) Open(ctx context.Context) (api.Gateway, jsonrpc.ClientCloser, error) {
	return client.NewGatewayRPCV1(ctx, o.addr, o.headers)
}

func NewGatewayOpener(ctx *cli.Context) (*GatewayOpener, APICloser, error) {

	tokenMaddr := ctx.String("gateway-api")

	apiInfo := cliutil.ParseApiInfo(tokenMaddr)

	addr, err := apiInfo.DialArgs("v1")
	if err != nil {
		return nil, nil, fmt.Errorf("parse listen address: %w", err)
	}

	o := &GatewayOpener{
		addr:    addr,
		headers: apiInfo.AuthHeader(),
	}

	return o, APICloser(func() {}), nil
}
