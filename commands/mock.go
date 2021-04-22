package commands

import (
	"context"
	"time"

	"github.com/filecoin-project/dealbot/testutil"
	"github.com/urfave/cli/v2"
)

var MockCmd = &cli.Command{
	Name:   "mock",
	Usage:  "Start a mock dealbot daemon, simulating tasks over RPC",
	Flags:  append(EndpointFlags, MockFlags...),
	Action: mockCommand,
}

func mockCommand(c *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	srv := testutil.NewMockDaemon(ctx, c)

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

	log.Infow("listen and serve")
	err := srv.Serve()
	return err
}
