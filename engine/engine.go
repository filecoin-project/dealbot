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
	maxTaskLifetime = 24 * time.Hour
	noTasksWait     = 5 * time.Second
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
	if workers < 1 {
		workers = 1
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

	// Create context to cancel workers on Close
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
	// pop a task
	task, err := e.client.PopTask(ctx, tasks.Type.PopTask.Of(e.host, tasks.InProgress))
	if err != nil {
		log.Warnw("pop-task returned error", "err", err)
		return nil
	}
	if task == nil {
		return nil // no task available
	}
	if task.WorkedBy.Must().String() != e.host {
		log.Warnw("pop-task returned task that is not for this host", "err", err)
		return nil
	}
	return task // found a runable task
}

func (e *Engine) worker(ctx context.Context, n int) {
	defer e.wg.Done()

	log.Infow("engine worker started", "worker_id", n)

	for {
		// Check if there is a new task
		task := e.popTask(ctx)
		if task == nil {
			// No tasks to run, so wait
			select {
			case <-ctx.Done():
				return // engine closed, worker exits.
			case <-time.After(noTasksWait):
			}
			continue
		}

		log.Infow("successfully acquired task", "uuid", task.UUID.String(), "worker_id", n)
		e.runTask(ctx, task)
	}
}

func (e *Engine) runTask(ctx context.Context, task tasks.Task) {
	var err error

	// Define function to update task stage.  Use engine context, not tasks context.
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

	// Create a context to manage the lifetime of the current task
	taskCtx, taskCancel := context.WithTimeout(ctx, maxTaskLifetime)
	defer taskCancel()

	finalStatus := tasks.Successful

	tlog := log.With("uuid", task.UUID.String())

	// Start deals
	if task.RetrievalTask.Exists() {
		err = tasks.MakeRetrievalDeal(taskCtx, e.nodeConfig, e.node, task.RetrievalTask.Must(), updateStage, log.Infow)
		if err != nil {
			if err == context.Canceled {
				// Engine closed, do not update final state
				tlog.Warn("task abandoned for shutdown")
				return
			}
			finalStatus = tasks.Failed
			tlog.Errorw("retrieval task returned error", "err", err)
		} else {
			tlog.Info("successfully retrieved data")
		}
	} else if task.StorageTask.Exists() {
		err = tasks.MakeStorageDeal(taskCtx, e.nodeConfig, e.node, task.StorageTask.Must(), updateStage, log.Infow)
		if err != nil {
			if err == context.Canceled {
				// Engine closed, do not update final state
				tlog.Warn("task abandoned for shutdown")
				return
			}
			finalStatus = tasks.Failed
			tlog.Errorw("storage task returned error", "err", err)
		} else {
			tlog.Info("successfully stored data")
		}
	} else {
		tlog.Error("task has unknown deal type")
		return
	}

	// Update task final status. Do not use task context.
	_, err = e.client.UpdateTask(ctx, task.UUID.String(),
		tasks.Type.UpdateTask.OfStage(
			e.host,
			finalStatus,
			task.Stage.String(),
			task.CurrentStageDetails.Must(),
		))

	if err != nil {
		if err == context.Canceled {
			tlog.Warn("task abandoned for shutdown")
			return
		}
		tlog.Errorw("error updating final status", "err", err)
	}
}
