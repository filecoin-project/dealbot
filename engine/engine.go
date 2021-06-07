package engine

import (
	"context"
	"sync"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/scheduler"
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

	nodeConfig tasks.NodeConfig
	node       api.FullNode
	closer     lotus.NodeCloser
	sched      *scheduler.Scheduler
	shutdown   chan struct{}
	stopped    chan struct{}
	tags       []string
	workerPing chan struct{}

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

	e := &Engine{
		client:        client,
		nodeConfig:    nodeConfig,
		node:          node,
		closer:        closer,
		host:          host_id,
		sched:         scheduler.New(),
		shutdown:      make(chan struct{}),
		stopped:       make(chan struct{}),
		tags:          cliCtx.StringSlice("tags"),
		workerPing:    make(chan struct{}),
		stageTimeouts: stageTimeouts,
	}

	go e.run(workers)
	return e, nil
}

func (e *Engine) run(workers int) {
	defer close(e.stopped)
	var wg sync.WaitGroup

	// Start workers
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go e.worker(i, &wg)
	}

	popTimer := time.NewTimer(noTasksWait)
taskLoop:
	for {
		select {
		case <-e.shutdown:
			break taskLoop
		default:
		}

		check, _ := context.WithDeadline(context.Background(), time.Now().Add(3*time.Second))
		if e.pingWorker() && e.apiGood(check) {
			// Check if there is a new task
			task := e.popTask()
			if task != nil {
				// Get task's schedule.  Tasks that have an empty schedule are run once, immediately.
				taskSchedule, limit := getTaskSchedule(task)

				// Add task to scheduler
				log.Infow("scheduling task", "uuid", task.UUID.String(), "schedule", taskSchedule, "schedule_limit", limit)
				e.sched.Add(taskSchedule, task, maxTaskRunTime, limit, 0)
				continue
			}
		}

		// No tasks to run now, so wait for scheduler or timer
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
	wg.Wait()
}

func (e *Engine) Close(ctx context.Context) {
	close(e.shutdown)  // signal to stop workers
	e.sched.Close(ctx) // stop scheduler and wait for all running tasks to stop
	<-e.stopped        // wait for workers to stop
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

func (e *Engine) worker(n int, wg *sync.WaitGroup) {
	log.Infow("engine worker started", "worker_id", n)
	defer wg.Done()
	runNow := e.sched.RunChan()
	for {
		select {
		case job, ok := <-runNow:
			if !ok {
				return
			}
			e.runTask(job.Context, job.Task, job.RunCount, n)
			job.End()
		case <-e.workerPing:
		}
	}
}

func (e *Engine) runTask(ctx context.Context, task tasks.Task, runCount, worker int) {
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
		stageDetails = tasks.BlankStage
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
}

func getTaskSchedule(task tasks.Task) (string, time.Duration) {
	var schedule, limit string
	var duration time.Duration

	if t := task.RetrievalTask; t.Exists() {
		if sch := t.Must().Schedule; sch.Exists() {
			schedule = sch.Must().String()
			if lim := t.Must().ScheduleLimit; lim.Exists() {
				limit = lim.Must().String()
			}
		}
	}

	if t := task.StorageTask; t.Exists() {
		if sch := t.Must().Schedule; sch.Exists() {
			schedule = sch.Must().String()
			if lim := t.Must().ScheduleLimit; lim.Exists() {
				limit = lim.Must().String()
			}
		}
	}

	if schedule != "" && limit != "" {
		var err error
		duration, err = time.ParseDuration(limit)
		if err != nil {
			log.Errorw("task has invalid value for ScheduleLimit", "uuid", task.UUID.String(), "err", err)
		}
	}

	return schedule, duration
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
func (e *Engine) apiGood(ctx context.Context) bool {
	localBalance, err := e.node.WalletBalance(ctx, e.nodeConfig.WalletAddress)
	if err != nil {
		log.Errorw("could not query api for wallet balance", "error", err)
		return false
	}
	if localBalance.LessThan(e.nodeConfig.MinWalletBalance) {
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
	if localCap == nil {
		return e.nodeConfig.MinWalletCap.NilOrZero()
	}
	if localCap.LessThan(e.nodeConfig.MinWalletCap) {
		return false
	}

	return true
}
