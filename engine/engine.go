package engine

import (
	"context"
	"sync"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/lotus/api"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"

	logging "github.com/ipfs/go-log/v2"
)

const (
	nextTaskTimeout = 10 * time.Second
	noTasksWait     = 5 * time.Second
	maxTaskLifetime = 24 * time.Hour

	defaultWorkers = 1
)

var log = logging.Logger("engine")

type Engine struct {
	host   string
	client *client.Client

	nodeConfig tasks.NodeConfig
	node       api.FullNode
	closer     lotus.NodeCloser

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(ctx context.Context, cliCtx *cli.Context) (*Engine, error) {
	workers := cliCtx.Int("workers")
	if workers == 0 {
		workers = defaultWorkers
	}

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

	ctx, e.cancel = context.WithCancel(ctx)

	e.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go e.worker(ctx, i)
	}

	return e, nil
}

func (e *Engine) Close() {
	e.cancel()  // stop workers
	e.wg.Wait() // wait for workers to stop
	e.closer()
}

func (e *Engine) popTask(ctx context.Context) tasks.Task {
	ctx, cancel := context.WithTimeout(ctx, nextTaskTimeout)
	defer cancel()

	// Pop tasks until a task is found or until no more tasks
	for {
		if ctx.Err() != nil {
			return nil // no task found before deadline
		}
		// pop a task
		task, err := e.client.PopTask(ctx, tasks.Type.PopTask.Of(e.host, tasks.InProgress))
		if err != nil {
			if err == context.DeadlineExceeded {
				return nil // no task found before deadline
			}
			log.Warnw("pop-task returned error", "err", err)
			continue
		}
		if task == nil {
			return nil // no task available
		}
		if task.WorkedBy.Must().String() != e.host {
			log.Warnw("pop-task returned task that is not for this host", "err", err)
			continue
		}
		return task // found a runable task
	}
}

func (e *Engine) worker(ctx context.Context, n int) {
	defer e.wg.Done()

	log.Infow("engine worker started", "worker_id", n)

	var taskCtx context.Context
	var taskCancel context.CancelFunc
	for {
		// Check if there is a new tasks.
		task := e.popTask(ctx)
		if task == nil {
			// No tasks to run, so wait
			select {
			case <-ctx.Done():
				return
			case <-time.After(noTasksWait):
				continue
			}
		}

		log.Infow("successfully acquired task", "uuid", task.UUID)

		// Create a context to manage the lifetime of the current task
		taskCtx, taskCancel = context.WithTimeout(ctx, maxTaskLifetime)

		e.runTask(taskCtx, task)
		taskCancel()
	}
}

func (e *Engine) runTask(ctx context.Context, task tasks.Task) {
	var err error

	// Define function to update task stage
	// TODO: updateState func should task context, passed in by deal executor.
	updateStage := func(stage string, stageDetails tasks.StageDetails) error {
		task, err = e.client.UpdateTask(ctx, task.UUID.String(),
			tasks.Type.UpdateTask.OfStage(
				e.host,
				tasks.InProgress,
				stage,
				stageDetails,
			))
		return err
	}

	finalStatus := tasks.Successful

	// Start deals
	if task.RetrievalTask.Exists() {
		err = tasks.MakeRetrievalDeal(ctx, e.nodeConfig, e.node, task.RetrievalTask.Must(), updateStage, log.Infow)
		if err != nil {
			if err == context.Canceled {
				log.Warn("task", task.UUID.String(), "canceled")
				return
			}
			finalStatus = tasks.Failed
			log.Errorw("retrieval task returned error", "err", err)
		} else {
			log.Info("successfully retrieved data")
		}
	}
	if task.StorageTask.Exists() {
		err = tasks.MakeStorageDeal(ctx, e.nodeConfig, e.node, task.StorageTask.Must(), updateStage, log.Infow)
		if err != nil {
			if err == context.Canceled {
				log.Warn("task", task.UUID.String(), "canceled")
				return
			}
			finalStatus = tasks.Failed
			log.Errorw("storage task returned error", "err", err)
		} else {
			log.Info("successfully stored data")
		}
	}

	_, err = e.client.UpdateTask(ctx, task.UUID.String(),
		tasks.Type.UpdateTask.OfStage(
			e.host,
			finalStatus,
			task.Stage.String(),
			task.CurrentStageDetails.Must(),
		))

	if err != nil {
		if err == context.Canceled {
			log.Warn("task", task.UUID.String(), "canceled")
			return
		}
		log.Error("Error updating final status")
	}
}
