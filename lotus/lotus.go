package lotus

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/api/v0api"
	cliutil "github.com/filecoin-project/lotus/cli/util"
	"github.com/filecoin-project/lotus/node/repo"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("dealbot")

type APIOpener struct {
	addr    string
	headers http.Header
}

type APICloser func()

func NewAPIOpenerFromCLI(cctx *cli.Context) (*APIOpener, APICloser, error) {

	var apiInfo cliutil.APIInfo

	if cctx.IsSet("api") {
		tokenMaddr := cctx.String("api")
		apiInfo = cliutil.ParseApiInfo(tokenMaddr)
	} else if cctx.IsSet("lotus-path") {
		repoPath := cctx.String("lotus-path")
		p, err := homedir.Expand(repoPath)
		if err != nil {
			return nil, nil, fmt.Errorf("expand home dir (%s): %w", repoPath, err)
		}

		r, err := repo.NewFS(p)
		if err != nil {
			return nil, nil, fmt.Errorf("open repo at path: %s; %w", p, err)
		}

		ma, err := r.APIEndpoint()
		if err != nil {
			return nil, nil, fmt.Errorf("api endpoint: %w", err)
		}

		token, err := r.APIToken()
		if err != nil {
			return nil, nil, fmt.Errorf("api token: %w", err)
		}

		apiInfo.Addr = ma.String()
		apiInfo.Token = token
	} else {
		return nil, nil, fmt.Errorf("cannot connect to lotus api: missing --api or --repo flags")
	}

	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, fmt.Errorf("parse listen address: %w", err)
	}

	o := &APIOpener{
		addr:    addr,
		headers: apiInfo.AuthHeader(),
	}

	return o, APICloser(func() {}), nil
}

func (o *APIOpener) Open(ctx context.Context) (v0api.FullNode, jsonrpc.ClientCloser, error) {
	return client.NewFullNodeRPCV0(ctx, o.addr, o.headers)
}

func setupLogging(cctx *cli.Context) error {
	ll := cctx.String("log-level")
	if err := logging.SetLogLevel("*", ll); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}

	if err := logging.SetLogLevel("rpc", "error"); err != nil {
		return fmt.Errorf("set rpc log level: %w", err)
	}

	llnamed := cctx.String("log-level-named")
	if llnamed == "" {
		return nil
	}

	for _, llname := range strings.Split(llnamed, ",") {
		parts := strings.Split(llname, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid named log level format: %q", llname)
		}
		if err := logging.SetLogLevel(parts[0], parts[1]); err != nil {
			return fmt.Errorf("set named log level %q to %q: %w", parts[0], parts[1], err)
		}

	}

	return nil
}

func NewAPIOpener(ctx *cli.Context) (*APIOpener, APICloser, error) {

	tokenMaddr := ctx.String("api")

	apiInfo := cliutil.ParseApiInfo(tokenMaddr)

	addr, err := apiInfo.DialArgs("v0")
	if err != nil {
		return nil, nil, fmt.Errorf("parse listen address: %w", err)
	}

	o := &APIOpener{
		addr:    addr,
		headers: apiInfo.AuthHeader(),
	}

	return o, APICloser(func() {}), nil
}
