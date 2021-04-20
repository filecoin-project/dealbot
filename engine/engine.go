package engine

import (
	"context"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/lotus/api"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("engine")

type Engine struct {
	host   string
	client *client.Client

	nodeConfig tasks.NodeConfig
	node       api.FullNode
	closer     lotus.NodeCloser
}

func New(ctx context.Context, cliCtx *cli.Context) (*Engine, error) {
	workers := 1

	client := client.New(cliCtx)

	nodeConfig, node, closer, err := lotus.SetupClient(ctx, cliCtx)
	if err != nil {
		return nil, err
	}

	v, err := node.Version(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("remote version: %s", v.Version)

	e := &Engine{
		client:     client,
		nodeConfig: nodeConfig,
		node:       node,
		closer:     closer,
		host:       uuid.New().String()[:8], // TODO: set from config toml
	}

	for i := 0; i < workers; i++ {
		go e.worker(i)
	}

	return e, nil
}

func (e *Engine) Close() {
	e.closer()
}

func (e *Engine) worker(n int) {
	log.Infow("engine worker started", "worker_id", n)

	for {
		// add delay to avoid querying the controller many times if there are no available tasks
		time.Sleep(5 * time.Second)

		// fetch tasks
		ctx := context.Background()
		alltasks, err := e.client.ListTasks(ctx)
		if err != nil {
			log.Warnw("list tasks returned error", "err", err)
			continue
		}

		// log tasks
		for _, t := range alltasks {
			t.Log(log)
		}

		// pick first available task
		var task *tasks.Task
		for _, t := range alltasks {
			if t.Status != tasks.Available {
				continue
			}
			task = t
			break
		}
		if task == nil {
			continue // no task available
		}

		req := &client.UpdateTaskRequest{
			Status:   2,
			WorkedBy: e.host,
		}

		task, err = e.client.UpdateTask(ctx, task.UUID, req)
		if err != nil {
			log.Warnw("update task returned error", "err", err)
			continue
		}

		log.Infow("successfully acquired task", "uuid", task.UUID)

		if task.RetrievalTask != nil {
			ctx := context.TODO()
			err = tasks.MakeRetrievalDeal(ctx, e.nodeConfig, e.node, *task.RetrievalTask, func(msg string, keysAndValues ...interface{}) {
				log.Infow(msg, keysAndValues...)
			})
			if err != nil {
				log.Errorw("retrieval task returned error", "err", err)
				continue
			}

			log.Info("successfully retrieved data")
		}

		if task.StorageTask != nil {
			ctx := context.TODO()
			err = tasks.MakeStorageDeal(ctx, e.nodeConfig, e.node, *task.StorageTask, func(msg string, keysAndValues ...interface{}) {
				log.Infow(msg, keysAndValues...)
			})
			if err != nil {
				log.Errorw("storage task returned error", "err", err)
				continue
			}

			log.Info("successfully stored data")
		}
	}
}
