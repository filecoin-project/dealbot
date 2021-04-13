package commands

import (
	"context"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/config"
	"github.com/filecoin-project/dealbot/daemon"
	"github.com/urfave/cli/v2"
)

var DaemonCmd = &cli.Command{
	Name:  "daemon",
	Usage: "Start a dealbot daemon, accepting tasks over RPC",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "configpath",
			Usage: "path to config toml file",
			Value: ".dealbot.env.toml",
		},
	},
	Action: daemonCommand,
}

func daemonCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	configpath := c.String("configpath")

	cfg := &config.EnvConfig{}
	if err := cfg.Load(configpath); err != nil {
		return err
	}
	if err := cfg.OverrideFromEnv(c); err != nil {
		return err
	}

	srv, err := daemon.New(ctx, cfg)
	if err != nil {
		return err
	}

	exiting := make(chan struct{})
	defer close(exiting)

	go func() {
		select {
		case <-ctx.Done():
		case <-exiting:
			// no need to shutdown in this case.
			return
		}

		log.Infow("shutting down rpc server")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalw("failed to shut down rpc server", "err", err)
		}
		log.Infow("rpc server stopped")
	}()

	log.Infow("listen and serve", "addr", srv.Addr())
	err = srv.Serve()
	if err == http.ErrServerClosed {
		err = nil
	}
	return err
}
