package commands

import (
	"time"

	_ "github.com/lib/pq"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("dealbot")

var CommonFlags []cli.Flag = []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "wallet",
		Usage:   "deal client wallet address on node",
		Aliases: []string{"w"},
		EnvVars: []string{"DEALBOT_WALLET_ADDRESS"},
	}),
}

var DealFlags = []cli.Flag{
	altsrc.NewPathFlag(&cli.PathFlag{
		Name:     "data-dir",
		Usage:    "writable directory used to transfer data to node",
		Aliases:  []string{"d"},
		EnvVars:  []string{"DEALBOT_DATA_DIRECTORY"},
		Required: true,
	}),
	altsrc.NewPathFlag(&cli.PathFlag{
		Name:    "node-data-dir",
		Usage:   "data-dir from relative to node's location [data-dir]",
		Aliases: []string{"n"},
		EnvVars: []string{"DEALBOT_NODE_DATA_DIRECTORY"},
	}),
}

var MinerFlags = []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:     "miner",
		Usage:    "address of miner to make deal with",
		Aliases:  []string{"m"},
		EnvVars:  []string{"DEALBOT_MINER_ADDRESS"},
		Required: true,
	}),
}

var RetrievalFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  "cid",
		Usage: "payload cid to fetch from miner",
		Value: "bafykbzacedikkmeotawrxqquthryw3cijaonobygdp7fb5bujhuos6wdkwomm",
	},
	altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
		Name:    "stage-timeout",
		Usage:   "stagename=duration exampple: DealAccepted=30m",
		EnvVars: []string{"STAGE_TIMEOUT"},
	}),
}

var StorageFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:    "fast-retrieval",
		Usage:   "request fast retrieval [true]",
		Aliases: []string{"f"},
		EnvVars: []string{"DEALBOT_FAST_RETRIEVAL"},
		Value:   true,
	},
	&cli.BoolFlag{
		Name:    "verified-deal",
		Usage:   "true if deal is verified [false]",
		Aliases: []string{"v"},
		EnvVars: []string{"DEALBOT_VERIFIED_DEAL"},
		Value:   false,
	},
	&cli.StringFlag{
		Name:    "size",
		Usage:   "size of deal (1KB, 2MB, 12GB, etc.) [1MB]",
		Aliases: []string{"s"},
		EnvVars: []string{"DEALBOT_DEAL_SIZE"},
		Value:   "1KB",
	},
	&cli.Int64Flag{
		Name:    "max-price",
		Usage:   "maximum Attofil to pay per byte per epoch []",
		EnvVars: []string{"DEALBOT_MAX_PRICE"},
		Value:   5e16,
	},
	&cli.Int64Flag{
		Name:    "start-offset",
		Usage:   "epochs deal start will be offset from now [30760 (10 days)]",
		EnvVars: []string{"DEALBOT_START_OFFSET"},
		Value:   30760,
	},
}

var SingleTaskFlags = append(DealFlags, MinerFlags...)

var EndpointFlags = []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:     "endpoint",
		Usage:    "HTTP endpoint of the controller",
		Aliases:  []string{"e"},
		EnvVars:  []string{"DEALBOT_CONTROLLER_ENDPOINT"},
		Required: true,
	}),
}

var DaemonFlags = append(DealFlags, append(CommonFlags, append(EndpointFlags, []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "id",
		Usage:   "set bot worker id",
		EnvVars: []string{"DEALBOT_ID"},
	}),
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "listen",
		Usage:   "host:port to bind http server on",
		Aliases: []string{"l"},
		EnvVars: []string{"DEALBOT_LISTEN"},
	}),
	altsrc.NewIntFlag(&cli.IntFlag{
		Name:    "workers",
		Usage:   "number of concurrent task workers",
		EnvVars: []string{"DEALBOT_WORKERS"},
		Value:   1,
	}),
	altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
		Name:    "tags",
		Usage:   "comma separated tag strings",
		EnvVars: []string{"DEALBOT_TAGS"},
	}),
	altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
		Name:    "stage-timeout",
		Usage:   "stagename=duration exampple: DealAccepted=30m",
		EnvVars: []string{"STAGE_TIMEOUT"},
	}),
}...)...)...)

var MockFlags = []cli.Flag{
	altsrc.NewFloat64Flag(&cli.Float64Flag{
		Name:    "success_rate",
		Usage:   "rate of deal successes (1.0 = full success, 0 = full failure)",
		Aliases: []string{"r"},
		EnvVars: []string{"MOCK_DEALBOT_SUCCESS_RATE"},
		Value:   0.5,
	}),
	altsrc.NewDurationFlag(&cli.DurationFlag{
		Name:    "success_avg",
		Usage:   "avergage time for a successful deal",
		Aliases: []string{"S"},
		EnvVars: []string{"MOCK_DEALBOT_SUCCESS_AVG"},
		Value:   1 * time.Minute,
	}),
	altsrc.NewDurationFlag(&cli.DurationFlag{
		Name:    "success_deviation",
		Usage:   "std deviation from average for successful deals",
		Aliases: []string{"s"},
		EnvVars: []string{"MOCK_DEALBOT_SUCCESS_DEVIATION"},
		Value:   1 * time.Second,
	}),
	altsrc.NewDurationFlag(&cli.DurationFlag{
		Name:    "failure_avg",
		Usage:   "avergage time for a failed deal",
		Aliases: []string{"F"},
		EnvVars: []string{"MOCK_DEALBOT_FAILURE_AVG"},
		Value:   2 * time.Minute,
	}),
	altsrc.NewDurationFlag(&cli.DurationFlag{
		Name:    "failure_deviation",
		Usage:   "std deviation from average for failed deals",
		Aliases: []string{"f"},
		EnvVars: []string{"MOCK_DEALBOT_FAILURE_AVG"},
		Value:   20 * time.Second,
	}),
	altsrc.NewIntFlag(&cli.IntFlag{
		Name:    "workers",
		Usage:   "number of simultaneous workers",
		Aliases: []string{"w"},
		EnvVars: []string{"MOCK_DEALBOT_WORKERS"},
		Value:   1,
	}),
}

var MockTaskFlags = []cli.Flag{
	altsrc.NewIntFlag(&cli.IntFlag{
		Name:    "count",
		Usage:   "number of mock tasks to generate",
		Aliases: []string{"c"},
		EnvVars: []string{"MOCK_DEALBOT_TASK_COUNT"},
		Value:   100,
	}),
	altsrc.NewIntFlag(&cli.IntFlag{
		Name:    "retrievals",
		Usage:   "number of mock tasks to should be retrievals",
		Aliases: []string{"r"},
		EnvVars: []string{"MOCK_DEALBOT_RETRIEVAL_COUNT"},
		Value:   0,
	}),
}
var ControllerFlags = []cli.Flag{
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "listen",
		Usage:   "host:port to bind http server on",
		Aliases: []string{"l"},
		EnvVars: []string{"DEALBOT_LISTEN"},
	}),
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "graphql",
		Usage:   "host:port to bind graphql server on",
		EnvVars: []string{"DEALBOT_GRAPHQL_LISTEN"},
	}),
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "metrics",
		Usage:   "value of 'prometheus' or 'log'",
		Aliases: []string{"m"},
		EnvVars: []string{"DEALBOT_METRICS"},
	}),
	altsrc.NewPathFlag(&cli.PathFlag{
		Name:    "identity",
		Usage:   "location of node identity for authenticating results",
		Aliases: []string{"i"},
		EnvVars: []string{"DEALBOT_IDENTITY_KEYPAIR"},
	}),
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "driver",
		Usage:   "type of database backend to use",
		EnvVars: []string{"DEALBOT_PERSISTENCE_DRIVER"},
		Value:   "sqlite",
	}),
	altsrc.NewStringFlag(&cli.StringFlag{
		Name:    "dbloc",
		Usage:   "connection string for sql DB",
		EnvVars: []string{"DEALBOT_PERSISTENCE_CONN"},
	}),
}

var AllFlags = append(DealFlags, append(SingleTaskFlags, append(DaemonFlags, append(ControllerFlags, MockFlags...)...)...)...)
