package engine

import (
	"context"
	"os/exec"
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
	maxTaskRunTime = 24 * time.Hour
	noTasksWait    = 5 * time.Second
	popTaskTimeout = 2 * time.Second
)

var log = logging.Logger("engine")

type Engine struct {
	host   string
	client *client.Client

	nodeConfig  tasks.NodeConfig
	node        api.FullNode
	closer      lotus.NodeCloser
	shutdown    chan struct{}
	stopped     chan struct{}
	tags        []string
	workerPing  chan struct{}
	cancelTasks context.CancelFunc

	stageTimeouts map[string]time.Duration
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

	v, err := node.Version(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("remote version: %s", v.Version)

	// before we do anything, reset all this workers tasks
	err = client.ResetWorker(cliCtx.Context, host_id)
	if err != nil {
		// for now, just log an error if this happens... seems like there are scenarios
		// where we want to get the dealbot up and running even though reset worker failed for
		// whatever eason
		log.Errorf("error resetting tasks for worker: %s", err)
	}

	e := &Engine{
		client:        client,
		nodeConfig:    nodeConfig,
		node:          node,
		closer:        closer,
		host:          host_id,
		shutdown:      make(chan struct{}),
		stopped:       make(chan struct{}),
		tags:          cliCtx.StringSlice("tags"),
		workerPing:    make(chan struct{}),
		stageTimeouts: stageTimeouts,
	}

	var tasksCtx context.Context
	tasksCtx, e.cancelTasks = context.WithCancel(context.Background())

	go e.run(tasksCtx, workers)
	return e, nil
}

func (e *Engine) run(tasksCtx context.Context, workers int) {
	defer close(e.stopped)
	defer e.cancelTasks()

	var wg sync.WaitGroup
	runChan := make(chan tasks.Task)

	// Start workers
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go e.worker(tasksCtx, i, &wg, runChan)
	}

	popTimer := time.NewTimer(noTasksWait)
taskLoop:
	for {
		select {
		case <-e.shutdown:
			break taskLoop
		default:
		}

		if e.pingWorker() && e.apiGood() {
			// Check if there is a new task
			task := e.popTask()
			if task != nil {
				log.Infow("sending task to worker", "uuid", task.UUID.String())
				runChan <- task
				continue
			}
		}

		// No tasks to run now, so wait for timer
		select {
		case <-popTimer.C:
			// Time to check for more new tasks
			popTimer.Reset(noTasksWait)
		case <-e.shutdown:
			// Engine shutdown
			break taskLoop
		}
	}

	// Stop workers and wait for all workers to exit
	close(runChan)
	wg.Wait()
}

func (e *Engine) Close(ctx context.Context) {
	close(e.shutdown) // signal to stop workers
	select {
	case <-e.stopped: // wait for workers to stop
	case <-ctx.Done(): // if waiting too long
		e.cancelTasks() // cancel any running tasks
		<-e.stopped     // wait for stop
	}
	e.closer()
}

func (e *Engine) popTask() tasks.Task {
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

func (e *Engine) worker(ctx context.Context, n int, wg *sync.WaitGroup, runChan <-chan tasks.Task) {
	log.Infow("engine worker started", "worker_id", n)
	defer wg.Done()
	for {
		select {
		case task, ok := <-runChan:
			if !ok {
				return
			}
			e.runTask(ctx, task, n)
		case <-e.workerPing:
		}
	}
}

func (e *Engine) runTask(ctx context.Context, task tasks.Task, worker int) {
	// Create a context to manage the running time of the current task
	ctx, cancel := context.WithTimeout(ctx, maxTaskRunTime)
	defer cancel()

	runCount64, _ := task.RunCount.AsInt()
	runCount := int(runCount64) + 1
	var err error
	log.Infow("worker running task", "uuid", task.UUID.String(), "run_count", runCount, "worker_id", worker)

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
		err = tasks.MakeRetrievalDeal(ctx, e.nodeConfig, e.node, task.RetrievalTask.Must(), updateStage, log.Infow, e.stageTimeouts)
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
		err = tasks.MakeStorageDeal(ctx, e.nodeConfig, e.node, task.StorageTask.Must(), updateStage, log.Infow, e.stageTimeouts)
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

// pingWorker returns true if a worker is available to read the workerPing
// channel.  This does not guarantee that the worker will still be available
// after returning true if there are scheduled tasks that the scheduler may
// run.
func (e *Engine) pingWorker() bool {
	select {
	case e.workerPing <- struct{}{}:
		return true
	default:
		return false
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
