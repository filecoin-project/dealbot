package lotus

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/filecoin-project/dealbot/config"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/node/repo"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var log = logging.Logger("dealbot")

type APIOpener struct {
	addr    string
	headers http.Header
}

type APICloser func()

func NewAPIOpenerFromCLI(cctx *cli.Context) (*APIOpener, APICloser, error) {
	var rawaddr, rawtoken string

	if cctx.IsSet("api") {
		tokenMaddr := cctx.String("api")
		toks := strings.Split(tokenMaddr, ":")
		if len(toks) != 2 {
			return nil, nil, fmt.Errorf("invalid api tokens, expected <token>:<maddr>, got: %s", tokenMaddr)
		}

		rawtoken = toks[0]
		rawaddr = toks[1]
	} else if cctx.IsSet("lotus-path") {
		repoPath := cctx.String("lotus-path")
		p, err := homedir.Expand(repoPath)
		if err != nil {
			return nil, nil, xerrors.Errorf("expand home dir (%s): %w", repoPath, err)
		}

		r, err := repo.NewFS(p)
		if err != nil {
			return nil, nil, xerrors.Errorf("open repo at path: %s; %w", p, err)
		}

		ma, err := r.APIEndpoint()
		if err != nil {
			return nil, nil, xerrors.Errorf("api endpoint: %w", err)
		}

		token, err := r.APIToken()
		if err != nil {
			return nil, nil, xerrors.Errorf("api token: %w", err)
		}

		rawaddr = ma.String()
		rawtoken = string(token)
	} else {
		return nil, nil, xerrors.Errorf("cannot connect to lotus api: missing --api or --repo flags")
	}

	parsedAddr, err := ma.NewMultiaddr(rawaddr)
	if err != nil {
		return nil, nil, xerrors.Errorf("parse listen address: %w", err)
	}

	_, addr, err := manet.DialArgs(parsedAddr)
	if err != nil {
		return nil, nil, xerrors.Errorf("dial multiaddress: %w", err)
	}

	o := &APIOpener{
		addr:    apiURI(addr),
		headers: apiHeaders(rawtoken),
	}

	return o, APICloser(func() {}), nil
}

func (o *APIOpener) Open(ctx context.Context) (api.FullNode, jsonrpc.ClientCloser, error) {
	return client.NewFullNodeRPCV1(ctx, o.addr, o.headers)
}

func apiURI(addr string) string {
	return "ws://" + addr + "/rpc/v0"
}

func apiHeaders(token string) http.Header {
	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+token)
	return headers
}

func setupLogging(cctx *cli.Context) error {
	ll := cctx.String("log-level")
	if err := logging.SetLogLevel("*", ll); err != nil {
		return xerrors.Errorf("set log level: %w", err)
	}

	if err := logging.SetLogLevel("rpc", "error"); err != nil {
		return xerrors.Errorf("set rpc log level: %w", err)
	}

	llnamed := cctx.String("log-level-named")
	if llnamed == "" {
		return nil
	}

	for _, llname := range strings.Split(llnamed, ",") {
		parts := strings.Split(llname, ":")
		if len(parts) != 2 {
			return xerrors.Errorf("invalid named log level format: %q", llname)
		}
		if err := logging.SetLogLevel(parts[0], parts[1]); err != nil {
			return xerrors.Errorf("set named log level %q to %q: %w", parts[0], parts[1], err)
		}

	}

	return nil
}

func NewAPIOpener(cfg *config.EnvConfig) (*APIOpener, APICloser, error) {
	var rawaddr, rawtoken string

	tokenMaddr := cfg.Daemon.API
	toks := strings.Split(tokenMaddr, ":")
	if len(toks) != 2 {
		return nil, nil, fmt.Errorf("invalid api tokens, expected <token>:<maddr>, got: %s", tokenMaddr)
	}

	rawtoken = toks[0]
	rawaddr = toks[1]

	parsedAddr, err := ma.NewMultiaddr(rawaddr)
	if err != nil {
		return nil, nil, xerrors.Errorf("parse listen address: %w", err)
	}

	_, addr, err := manet.DialArgs(parsedAddr)
	if err != nil {
		return nil, nil, xerrors.Errorf("dial multiaddress: %w", err)
	}

	o := &APIOpener{
		addr:    apiURI(addr),
		headers: apiHeaders(rawtoken),
	}

	return o, APICloser(func() {}), nil
}
