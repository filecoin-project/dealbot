package commands

import (
	"context"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/controller"
	"github.com/urfave/cli/v2"
)

var ControllerCmd = &cli.Command{
	Name:   "controller",
	Usage:  "Start the dealbot controller",
	Flags:  ControllerFlags,
	Action: controllerCommand,
}

func controllerCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	srv, err := controller.New(c)
	if err != nil {
		return err
	}

	exiting := make(chan struct{})
	defer close(exiting)

	go func() {
		select {
		case <-ctx.Done():
		case <-exiting:
			// no need to shut down in this case.
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
