package commands

import (
	"strings"

	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/version"

	logging "github.com/ipfs/go-log/v2"
	_ "github.com/lib/pq"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"
)

var log = logging.Logger("dealbot")

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

	log.Infof("Dealbot version:%s", version.String())

	return nil
}

func setupLotusAPI(cctx *cli.Context) (*lotus.APIOpener, lotus.APICloser, error) {
	return lotus.NewAPIOpener(cctx, 50_000)
}
