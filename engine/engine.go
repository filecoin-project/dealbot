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
)

var log = logging.Logger("engine")

type Engine struct {
	host   string
	client *client.Client

	nodeConfig tasks.NodeConfig
	node       api.FullNode
	closer     lotus.NodeCloser

	closeWait context.Context
	stop      context.CancelFunc
	stopped   chan struct{}
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
		stopped:    make(chan struct{}),
	}

	// Create context to cancel workers on shutdown, letting them finish
	// currently executing tasks
	ctx, e.stop = context.WithCancel(ctx)

	go e.run(ctx, workers)
	return e, nil
}

func (e *Engine) run(ctx context.Context, workers int) {
	defer close(e.stopped)
	var wg sync.WaitGroup

	sched := scheduler.New(ctx)

	// Start workers
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go e.worker(ctx, i, &wg, sched.RunChan())
	}

	popTimer := time.NewTimer(noTasksWait)
	for {
		if ctx.Err() != nil {
			break
		}

		// Check if there is a new task
		task := e.popTask(ctx)
		if task != nil {
			taskSchedule, limit := getTaskSchedule(task)

			// If task has no schedule, run now
			if taskSchedule == "" {
				taskSchedule = scheduler.RunNow
			}
			// Add task to scheduler
			log.Infow("scheduling task", "uuid", task.UUID.String(), "schedule", taskSchedule, "schedule_limit", limit)
			sched.Add(taskSchedule, task, maxTaskRunTime, limit)
			continue
		}

		// No tasks to run now, so wait for scheduler or timer
		select {
		case <-popTimer.C:
			// Time to check for more new tasks
			popTimer.Reset(noTasksWait)
		case <-ctx.Done():
			// Engine shutdown
			break
		}
	}

	// Stop scheduler and wait for all currently running tasks to stop
	sched.Stop(e.closeWait)

	// Stop workers and wait for all workers to exit
	wg.Wait()
	return
}

func (e *Engine) Close(ctx context.Context) {
	e.closeWait = ctx // false to let workers finish current task
	e.stop()          // stop workers and scheduler
	<-e.stopped       // wait for workers to stop
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

	log.Infow("successfully acquired task", "uuid", task.UUID.String())
	return task // found a runable task
}

func (e *Engine) worker(ctx context.Context, n int, wg *sync.WaitGroup, runNow <-chan *scheduler.Job) {
	defer wg.Done()
	log.Infow("engine worker started", "worker_id", n)
	for job := range runNow {
		e.runTask(ctx, job, n)
	}
}

func (e *Engine) runTask(ctx context.Context, job *scheduler.Job, n int) {
	defer job.End()

	var err error
	task := job.Task()

	log.Infow("worker running task", "uuid", task.UUID.String(), "run", job.RunCount(), "worker_id", n)

	// Define function to update task stage.  Use shutdown context, not task
	// context, so that if task is canceled after completing a stage, the
	// completion is still recodred.
	updateStage := func(stage string, stageDetails tasks.StageDetails) error {
		task, err = e.client.UpdateTask(ctx, task.UUID.String(),
			tasks.Type.UpdateTask.OfStage(
				e.host,
				tasks.InProgress,
				stage,
				stageDetails,
			), job.RunCount())
		return err
	}

	finalStatus := tasks.Successful

	tlog := log.With("uuid", task.UUID.String())

	// Start deals
	if task.RetrievalTask.Exists() {
		err = tasks.MakeRetrievalDeal(job.RunContext(), e.nodeConfig, e.node, task.RetrievalTask.Must(), updateStage, log.Infow)
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
		err = tasks.MakeStorageDeal(job.RunContext(), e.nodeConfig, e.node, task.StorageTask.Must(), updateStage, log.Infow)
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
		), job.RunCount())

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

	if task.RetrievalTask.Exists() {
		schedule = task.RetrievalTask.Must().Schedule.Must().String()
		limit = task.StorageTask.Must().ScheduleLimit.Must().String()
	}

	if task.StorageTask.Exists() {
		schedule = task.StorageTask.Must().Schedule.Must().String()
		limit = task.StorageTask.Must().ScheduleLimit.Must().String()
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
