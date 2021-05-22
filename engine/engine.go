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

	defaultProposeRetrievalTimeout = 30 * time.Minute
	defaultDealAcceptedTimeout     = 30 * time.Minute
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

	statusTimeouts map[string]time.Duration
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

	client := client.New(cliCtx)

	nodeConfig, node, closer, err := lotus.SetupClient(ctx, cliCtx)
	if err != nil {
		return nil, err
	}

	v, err := node.Version(ctx)
	if err != nil {
		return nil, err
	}

	// Establish default timeouts
	stageTimeouts := map[string]time.Duration{
		"ProposeRetrieval": defaultProposeRetrievalTimeout,
		"DealAccepted":     defaultDealAcceptedTimeout,
	}

	log.Infof("remote version: %s", v.Version)

	e := &Engine{
		client:     client,
		nodeConfig: nodeConfig,
		node:       node,
		closer:     closer,
		host:       host_id,
		sched:      scheduler.New(),
		shutdown:   make(chan struct{}),
		stopped:    make(chan struct{}),
		tags:       cliCtx.StringSlice("tags"),
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

		// Check if there is a new task
		task := e.popTask()
		if task != nil {
			taskSchedule, limit := getTaskSchedule(task)

			// If task has no schedule, run now
			if taskSchedule == "" {
				taskSchedule = scheduler.RunNow
			}
			// Add task to scheduler
			log.Infow("scheduling task", "uuid", task.UUID.String(), "schedule", taskSchedule, "schedule_limit", limit)
			e.sched.Add(taskSchedule, task, maxTaskRunTime, limit, 0)
			continue
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
	for job := range runNow {
		e.runTask(job.Context, job.Task, job.RunCount, n)
		job.End()
	}
}

func (e *Engine) runTask(ctx context.Context, task tasks.Task, runCount, worker int) {
	var err error
	log.Infow("worker running task", "uuid", task.UUID.String(), "run_count", runCount, "worker_id", worker)

	// Define function to update task stage.  Use shutdown context, not task
	updateStage := func(stage string, stageDetails tasks.StageDetails) error {
		updatedTask, err := e.client.UpdateTask(ctx, task.UUID.String(),
			tasks.Type.UpdateTask.OfStage(
				e.host,
				tasks.InProgress,
				"",
				stage,
				stageDetails,
				runCount,
			))
		// TODO: this is a weird side-effecty behavior and we should find a way
		// to remove it
		if err == nil {
			task = updatedTask
		}
		return err
	}

	finalStatus := tasks.Successful
	finalErrorMessage := ""
	tlog := log.With("uuid", task.UUID.String())

	// Start deals
	if task.RetrievalTask.Exists() {
		err = tasks.MakeRetrievalDeal(ctx, e.nodeConfig, e.node, task.RetrievalTask.Must(), updateStage, log.Infow)
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
		err = tasks.MakeStorageDeal(ctx, e.nodeConfig, e.node, task.StorageTask.Must(), updateStage, log.Infow)
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
