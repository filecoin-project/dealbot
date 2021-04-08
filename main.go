package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/filecoin-project/dealbot/commands"
	"github.com/filecoin-project/dealbot/version"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("dealbot")

func main() {
	// Set up a context that is canceled when the command is interrupted
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up a signal handler to cancel the context
	go func() {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, syscall.SIGTERM, syscall.SIGINT)
		select {
		case <-interrupt:
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := logging.SetLogLevel("*", "info"); err != nil {
		log.Fatal(err)
	}

	defaultName := "dealbot_" + version.String()
	hostname, err := os.Hostname()
	if err == nil {
		defaultName = fmt.Sprintf("%s_%s_%d", defaultName, hostname, os.Getpid())
	}

	app := &cli.App{
		Name:    "dealbot",
		Usage:   "Filecoin Network Storage and Retrieval Analysis Utility",
		Version: version.String(),
		Flags: append(commands.CommonFlags, []cli.Flag{
			&cli.StringFlag{
				Name:    "api",
				EnvVars: []string{"FULLNODE_API_INFO"},
				Value:   "",
			},
			&cli.StringFlag{
				Name:    "log-level",
				EnvVars: []string{"GOLOG_LOG_LEVEL"},
				Value:   "debug",
				Usage:   "Set the default log level for all loggers to `LEVEL`",
			},
			&cli.StringFlag{
				Name:    "log-level-named",
				EnvVars: []string{"DEALBOT_LOG_LEVEL_NAMED"},
				Value:   "",
				Usage:   "A comma delimited list of named loggers and log levels formatted as name:level, for example 'logger1:debug,logger2:info'",
			},
		}...),
		Commands: []*cli.Command{
			commands.MakeStorageDealCmd,
			commands.MakeRetrievalDealCmd,
			commands.DaemonCmd,
			commands.ControllerCmd,
			//commands.ClientCmd,
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
