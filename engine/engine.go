package engine

import (
	"context"
	"math"
	"os/exec"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/lotus/api"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"

	logging "github.com/ipfs/go-log/v2"
)

const (
	maxTaskRunTime = 24 * time.Hour
	noTasksWait    = 5 * time.Second
	popTaskTimeout = 2 * time.Second
)

var log = logging.Logger("engine")

type apiClient interface {
	GetTask(ctx context.Context, uuid string) (tasks.Task, error)
	UpdateTask(ctx context.Context, uuid string, r tasks.UpdateTask) (tasks.Task, error)
	PopTask(ctx context.Context, r tasks.PopTask) (tasks.Task, error)
	ResetWorker(ctx context.Context, worker string) error
}

type taskExecutor interface {
	MakeStorageDeal(ctx context.Context, config tasks.NodeConfig, node api.FullNode, task tasks.StorageTask, updateStage tasks.UpdateStage, log tasks.LogStatus, stageTimeouts map[string]time.Duration, releaseWorker func()) error
	MakeRetrievalDeal(ctx context.Context, config tasks.NodeConfig, node api.FullNode, task tasks.RetrievalTask, updateStage tasks.UpdateStage, log tasks.LogStatus, stageTimeouts map[string]time.Duration, releaseWorker func()) error
}

type defaultTaskExecutor struct{}

func (defaultTaskExecutor) MakeStorageDeal(ctx context.Context, config tasks.NodeConfig, node api.FullNode, task tasks.StorageTask, updateStage tasks.UpdateStage, log tasks.LogStatus, stageTimeouts map[string]time.Duration, releaseWorker func()) error {
	return tasks.MakeStorageDeal(ctx, config, node, task, updateStage, log, stageTimeouts, releaseWorker)
}

func (defaultTaskExecutor) MakeRetrievalDeal(ctx context.Context, config tasks.NodeConfig, node api.FullNode, task tasks.RetrievalTask, updateStage tasks.UpdateStage, log tasks.LogStatus, stageTimeouts map[string]time.Duration, releaseWorker func()) error {
	return tasks.MakeRetrievalDeal(ctx, config, node, task, updateStage, log, stageTimeouts, releaseWorker)
}

type Engine struct {
	shutdown     chan struct{}
	stopped      chan struct{}
	queueIsEmpty chan struct{}

	host          string
	tags          []string
	stageTimeouts map[string]time.Duration

	// depedencies
	node         api.FullNode
	nodeConfig   tasks.NodeConfig
	closer       lotus.NodeCloser
	client       apiClient
	clock        clock.Clock
	taskExecutor taskExecutor
}

func New(ctx context.Context, cliCtx *cli.Context) (*Engine, error) {
	workers := cliCtx.Int("workers")
	if workers < 1 {
		workers = 1
	}

	host_id := cliCtx.String("id")
	if host_id == "" {
		host_id = uuid.New().String()[:8]
	}

	stageTimeouts, err := tasks.ParseStageTimeouts(cliCtx.StringSlice("stage-timeout"))
	if err != nil {
		return nil, err
	}

	client := client.New(cliCtx)

	nodeConfig, node, closer, err := lotus.SetupClient(ctx, cliCtx)
	if err != nil {
		return nil, err
	}

	clock := clock.New()

	tags := cliCtx.StringSlice("tags")

	engine, err := new(ctx, host_id, stageTimeouts, tags, node, nodeConfig, closer, client, clock, defaultTaskExecutor{}, nil)
	if err != nil {
		return nil, err
	}

	go engine.run(ctx, workers)
	return engine, nil
}

// used for testing
func new(
	ctx context.Context,
	host string,
	stageTimeouts map[string]time.Duration,
	tags []string,
	node api.FullNode,
	nodeConfig tasks.NodeConfig,
	closer lotus.NodeCloser,
	client apiClient,
	clock clock.Clock,
	taskExecutor taskExecutor,
	queueIsEmpty chan struct{},
) (*Engine, error) {

	v, err := node.Version(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("remote version: %s", v.Version)

	// before we do anything, reset all this workers tasks
	err = client.ResetWorker(ctx, host)
	if err != nil {
		// for now, just log an error if this happens... seems like there are scenarios
		// where we want to get the dealbot up and running even though reset worker failed for
		// whatever eason
		log.Errorf("error resetting tasks for worker: %s", err)
	}

	return &Engine{
		client:        client,
		nodeConfig:    nodeConfig,
		node:          node,
		closer:        closer,
		host:          host,
		shutdown:      make(chan struct{}),
		stopped:       make(chan struct{}),
		tags:          tags,
		stageTimeouts: stageTimeouts,
		clock:         clock,
		taskExecutor:  taskExecutor,
		queueIsEmpty:  queueIsEmpty,
	}, nil
}

func (e *Engine) run(ctx context.Context, workers int) {
	defer close(e.stopped)

	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(ctx)

	e.taskLoop(ctx, wg, workers)
	cancel()

	// Stop workers and wait for all workers to exit
	wg.Wait()
}

func (e *Engine) taskLoop(ctx context.Context, wg sync.WaitGroup, workers int) {

	// super annoying -- make a new timer that is already stopped
	popTimer := e.clock.Timer(math.MinInt64)
	if !popTimer.Stop() {
		<-popTimer.C
	}

	ready := make(chan struct{}, 1)
	// we start in a ready state
	ready <- struct{}{}

	released := make(chan struct{}, workers)
	active := 0

	for {
		// insure at most one operation runs before a quit
		select {
		case <-e.shutdown:
			return
		default:
		}

		select {
		case <-e.shutdown:
			return
		case <-ready:
			// stop and drain timer if not already drained
			if !popTimer.Stop() {
				select {
				case <-popTimer.C:
				default:
				}
			}
			task := e.tryPopTask()
			if task != nil {
				active++
				wg.Add(1)
				go func() {
					defer wg.Done()
					e.runTask(ctx, task, 1, released)
				}()
				if active < workers {
					ready <- struct{}{}
				}
			} else {
				popTimer.Reset(noTasksWait)
				// only used for testing
				if e.queueIsEmpty != nil {
					e.queueIsEmpty <- struct{}{}
				}
			}
		case <-popTimer.C:
			// ready to queue next task if not otherwise queued
			select {
			case ready <- struct{}{}:
			default:
			}
		case <-released:
			active--
			// ready to queue next task if not otherwise queued
			select {
			case ready <- struct{}{}:
			default:
			}
		}
	}
}

func (e *Engine) Close(ctx context.Context) {
	close(e.shutdown) // signal to stop workers
	<-e.stopped       // wait for workers to stop
	e.closer()
}

func (e *Engine) tryPopTask() tasks.Task {
	if !e.apiGood() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), popTaskTimeout)
	defer cancel()

	// pop a task
	task, err := e.client.PopTask(ctx, tasks.Type.PopTask.Of(e.host, tasks.InProgress, e.tags...))
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

	log.Infow("successfully acquired task", "uuid", task.UUID.String())
	return task // found a runable task
}

func (e *Engine) runTask(ctx context.Context, task tasks.Task, runCount int, released chan<- struct{}) {
	var err error
	log.Infow("worker running task", "uuid", task.UUID.String(), "run_count", runCount)

	var releaseOnce sync.Once
	releaseWorker := func() {
		releaseOnce.Do(func() {
			released <- struct{}{}
		})
	}

	// Define function to update task stage.  Use shutdown context, not task
	updateStage := func(ctx context.Context, stage string, stageDetails tasks.StageDetails) error {
		_, err := e.client.UpdateTask(ctx, task.UUID.String(),
			tasks.Type.UpdateTask.OfStage(
				e.host,
				tasks.InProgress,
				"",
				stage,
				stageDetails,
				runCount,
			))
		return err
	}

	finalStatus := tasks.Successful
	finalErrorMessage := ""
	tlog := log.With("uuid", task.UUID.String())

	// Start deals
	if task.RetrievalTask.Exists() {
		err = e.taskExecutor.MakeRetrievalDeal(ctx, e.nodeConfig, e.node, task.RetrievalTask.Must(), updateStage, log.Infow, e.stageTimeouts, releaseWorker)
		if err != nil {
			if err == context.Canceled {
				// Engine closed, do not update final state
				tlog.Warn("task abandoned for shutdown")
				return
			}
			finalStatus = tasks.Failed
			finalErrorMessage = err.Error()
			tlog.Errorw("retrieval task returned error", "err", err)
		} else {
			tlog.Info("successfully retrieved data")
		}
	} else if task.StorageTask.Exists() {
		err = e.taskExecutor.MakeStorageDeal(ctx, e.nodeConfig, e.node, task.StorageTask.Must(), updateStage, log.Infow, e.stageTimeouts, releaseWorker)
		if err != nil {
			if err == context.Canceled {
				// Engine closed, do not update final state
				tlog.Warn("task abandoned for shutdown")
				return
			}
			finalStatus = tasks.Failed
			finalErrorMessage = err.Error()
			tlog.Errorw("storage task returned error", "err", err)
		} else {
			tlog.Info("successfully stored data")
		}
	} else {
		tlog.Error("task has unknown deal type")
		return
	}

	// Fetch task with all the updates from deal execution
	task, err = e.client.GetTask(ctx, task.UUID.String())
	if err != nil {
		tlog.Errorw("cannot get updated task to finalize", "err", err)
	}

	// if we haven't already released the queue to run more jobs, release it now
	releaseWorker()

	// Update task final status. Do not use task context.
	var stageDetails tasks.StageDetails
	if task.CurrentStageDetails.Exists() {
		stageDetails = task.CurrentStageDetails.Must()
	} else {
		stageDetails = tasks.BlankStage()
	}
	_, err = e.client.UpdateTask(ctx, task.UUID.String(),
		tasks.Type.UpdateTask.OfStage(
			e.host,
			finalStatus,
			finalErrorMessage,
			task.Stage.String(),
			stageDetails,
			runCount,
		))

	if err != nil {
		if err == context.Canceled {
			tlog.Warn("task abandoned for shutdown")
			return
		}
		tlog.Errorw("error updating final status", "err", err)
	}
	if e.nodeConfig.PostHook != "" {
		err := exec.Command("/bin/bash", e.nodeConfig.PostHook, task.GetUUID()).Run()
		if err != nil {
			tlog.Errorw("could not run post hook", "err", err)
		}
	}
}

// apiGood returns true if the api can be reached and reports sufficient fil/cap to process tasks.
func (e *Engine) apiGood() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	localBalance, err := e.node.WalletBalance(ctx, e.nodeConfig.WalletAddress)
	if err != nil {
		log.Errorw("could not query api for wallet balance", "error", err)
		return false
	}
	if localBalance.LessThan(e.nodeConfig.MinWalletBalance) {
		log.Warnf("Not accepting task. local balance below min")
		return false
	}

	head, err := e.node.ChainHead(ctx)
	if err != nil {
		log.Errorw("could not query api for head", "error", err)
		return false
	}
	localCap, err := e.node.StateVerifiedClientStatus(ctx, e.nodeConfig.WalletAddress, head.Key())
	if err != nil {
		log.Errorw("could not query api for datacap", "error", err)
		return false
	}
	log.Infow("local datacap", "datacap", localCap)

	if e.nodeConfig.MinWalletCap.Int64() < 0 {
		return true
	}

	if localCap == nil {
		return e.nodeConfig.MinWalletCap.NilOrZero()
	}
	if localCap.LessThan(e.nodeConfig.MinWalletCap) {
		log.Warnf("Not accepting task. local datacap below min")
		return false
	}

	return true
}
