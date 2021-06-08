package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/filecoin-project/dealbot/commands"
	"github.com/filecoin-project/dealbot/version"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
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

	appFlags := append(commands.CommonFlags, []cli.Flag{
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "api",
			EnvVars: []string{"FULLNODE_API_INFO"},
			Value:   "",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "lotus-path",
			EnvVars: []string{"LOTUS_PATH"},
			Value:   "",
			Usage:   "Set the lotus path instead of the API determine the api url automatically",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "log-level",
			EnvVars: []string{"GOLOG_LOG_LEVEL"},
			Value:   "debug",
			Usage:   "Set the default log level for all loggers to `LEVEL`",
		}),
		altsrc.NewStringFlag(&cli.StringFlag{
			Name:    "log-level-named",
			EnvVars: []string{"DEALBOT_LOG_LEVEL_NAMED"},
			Value:   "",
			Usage:   "A comma delimited list of named loggers and log levels formatted as name:level, for example 'logger1:debug,logger2:info'",
		}),
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
		},
	}...)
	app := &cli.App{
		Name:    "dealbot",
		Usage:   "Filecoin Network Storage and Retrieval Analysis Utility",
		Version: version.String(),
		Flags:   appFlags,
		Commands: []*cli.Command{
			commands.MakeStorageDealCmd,
			commands.MakeRetrievalDealCmd,
			commands.MockCmd,
			commands.MockTasksCmd,
			commands.DaemonCmd,
			commands.ControllerCmd,
			commands.QueueRetrievalCmd,
			commands.QueueStorageCmd,
		},
		Before: altsrc.InitInputSourceWithContext(append(appFlags, commands.AllFlags...), altsrc.NewYamlSourceFromFlagFunc("config")),
	}

	if _, ok := os.LookupEnv("DEALBOT_LOG_JSON"); ok {
		logging.SetupLogging(logging.Config{
			Format: logging.JSONOutput,
			Stdout: true,
		})
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
