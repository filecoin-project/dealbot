package testutil

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"

	"github.com/google/uuid"
)

var log = logging.Logger("mock_daemon")

// MockDaemon mimics a running daemon by pulling tasks of the queue and returning
// results
type MockDaemon struct {
	doneCh           chan struct{}
	forceClose       chan struct{}
	closed           chan struct{}
	successRate      float64
	failureAvg       time.Duration
	failureDeviation time.Duration
	successAvg       time.Duration
	successDeviation time.Duration
	client           *client.Client
	host             string
	workers          int
}

// NewMockDaemon initializes a new mocked out daemon
func NewMockDaemon(ctx context.Context, cliCtx *cli.Context) (srv *MockDaemon) {
	srv = new(MockDaemon)
	srv.client = client.New(cliCtx)
	srv.doneCh = make(chan struct{})
	srv.forceClose = make(chan struct{})
	srv.closed = make(chan struct{})
	srv.successRate = cliCtx.Float64("success_rate")
	srv.successAvg = cliCtx.Duration("success_avg")
	srv.successDeviation = cliCtx.Duration("success_deviation")
	srv.failureAvg = cliCtx.Duration("failure_avg")
	srv.failureDeviation = cliCtx.Duration("failure_deviation")
	srv.host = uuid.New().String()[:8]
	srv.workers = cliCtx.Int("workers")
	return srv
}

func (md *MockDaemon) worker(n int) {
	log.Infow("mock worker started", "worker_id", n)
	baseFailStates := make([]string, 0, len(tasks.ConnectivityStages))
	for state := range tasks.ConnectivityStages {
		baseFailStates = append(baseFailStates, state)
	}
	retrievalFailStates := make([]string, 0, len(tasks.RetrievalStages)-1+len(baseFailStates))
	for state := range tasks.RetrievalStages {
		if state != "DealComplete" {
			retrievalFailStates = append(retrievalFailStates, state)
		}
	}
	retrievalFailStates = append(retrievalFailStates, baseFailStates...)
	storageFailStates := make([]string, 0, len(storagemarket.DealStates)-1+len(baseFailStates))
	storageStageDetails := make(map[string]tasks.StageDetails, len(storagemarket.DealStates))
	for state, stateName := range storagemarket.DealStates {
		if state != storagemarket.StorageDealActive {
			storageFailStates = append(storageFailStates, stateName)
		}
		storageStageDetails[stateName] = tasks.Type.StageDetails.Of(storagemarket.DealStatesDescriptions[state], storagemarket.DealStatesDurations[state])
	}
	storageFailStates = append(storageFailStates, baseFailStates...)
	for {
		// add delay to avoid querying the controller many times if there are no available tasks
		time.Sleep(5 * time.Second)

		// pop a task
		ctx := context.Background()
		task, err := md.client.PopTask(ctx,
			tasks.Type.PopTask.Of(md.host, tasks.InProgress))
		if err != nil {
			log.Warnw("pop-task returned error", "err", err)
			continue
		}

		if task == nil {
			continue // no task available
		}

		if !task.WorkedBy.Exists() || (task.WorkedBy.Must().String() != md.host) {
			log.Warnw("pop-task returned a non-available task", "err", err)
			continue
		}

		log.Infow("successfully acquired task", "uuid", task.UUID)
		isSuccess := rand.Float64() <= md.successRate
		var taskDuration time.Duration
		var stage string
		var stageDetails tasks.StageDetails
		var ok bool
		if isSuccess {
			if task.RetrievalTask.Exists() {
				stage = "DealComplete"
				stageDetails = tasks.RetrievalStages[stage]
			} else {
				stage = "StorageDealActive"
				stageDetails = storageStageDetails[stage]
			}
			taskDuration = md.successAvg + time.Duration(rand.NormFloat64()*float64(md.successDeviation))
		} else {
			if task.RetrievalTask.Exists() {
				stage = retrievalFailStates[rand.Intn(len(retrievalFailStates))]
				stageDetails, ok = tasks.RetrievalStages[stage]
				if !ok {
					stageDetails = tasks.ConnectivityStages[stage]
				}
			} else {
				stage = storageFailStates[rand.Intn(len(storageFailStates))]
				stageDetails, ok = storageStageDetails[stage]
				if !ok {
					stageDetails = tasks.ConnectivityStages[stage]
				}
			}
			taskDuration = md.failureAvg + time.Duration(rand.NormFloat64()*float64(md.failureDeviation))
		}
		if taskDuration < 0 {
			taskDuration = 0
		}
		timer := time.NewTimer(taskDuration)
		select {
		case <-md.doneCh:
			return
		case <-timer.C:
			result := tasks.Successful
			errorMessage := ""
			if !isSuccess {
				result = tasks.Failed
				errorMessage = "Something terrible happened"
			}
			req := tasks.Type.UpdateTask.OfStage(
				md.host,
				result,
				errorMessage,
				stage,
				stageDetails,
				1,
			)

			task, err = md.client.UpdateTask(ctx, task.GetUUID(), req)
			if err != nil {
				log.Warnw("update task returned error", "err", err)
				continue
			}
		}
	}
}

func (md *MockDaemon) Serve() error {
	select {
	case <-md.doneCh:
		return fmt.Errorf("tried to reuse a stopped server")
	default:
	}

	var wg sync.WaitGroup
	for i := 0; i < md.workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			md.worker(i)
		}(i)
	}
	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
		close(md.closed)
	case <-md.forceClose:
		return fmt.Errorf("did not shutdown gracefully")
	}
	return nil
}

// Shutdown all workers
func (md *MockDaemon) Shutdown(ctx context.Context) error {
	close(md.doneCh)
	select {
	case <-md.closed:
	case <-ctx.Done():
		close(md.forceClose)
		return ctx.Err()
	}
	return nil
}
