package engine

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/filecoin-project/dealbot/lotus"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/mocks"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEngineTiming(t *testing.T) {
	ctx := context.Background()
	testCases := map[string]struct {
		testTasks    testTasks
		workers      int
		emptyDelayAt []time.Duration
	}{
		"schedules tasks in parallel up to max": {
			testTasks: testTasks{
				{
					availableAt:      0,
					expectedPoppedAt: 0,
					finishDuration:   100,
				},
				{
					availableAt:      0,
					expectedPoppedAt: 0,
					finishDuration:   10000,
				},
				{
					availableAt:      0,
					expectedPoppedAt: 0,
					finishDuration:   10000,
				},
				{
					availableAt:      0,
					expectedPoppedAt: 100,
					finishDuration:   10000,
				},
			},
			workers: 3,
		},
		"schedules tasks in parallel up to max, released early": {
			testTasks: testTasks{
				{
					availableAt:      0,
					expectedPoppedAt: 0,
					finishDuration:   100,
				},
				{
					availableAt:      0,
					expectedPoppedAt: 0,
					finishDuration:   100,
				},
				{
					availableAt:      0,
					expectedPoppedAt: 0,
					releaseDuration:  50,
					finishDuration:   100,
				},
				{
					availableAt:      0,
					expectedPoppedAt: 50,
					finishDuration:   100,
				},
			},
			workers: 3,
		},
		"pop on timeout up to max, released": {
			testTasks: testTasks{
				{
					availableAt:      100,
					expectedPoppedAt: noTasksWait,
					// finish total at noTasksWait*3+200
					finishDuration: noTasksWait*2 + 200,
				},
				{
					availableAt:      noTasksWait + 100,
					expectedPoppedAt: noTasksWait * 2,
					finishDuration:   noTasksWait*2 + 200,
				},
				{
					availableAt:      noTasksWait*2 + 100,
					expectedPoppedAt: noTasksWait * 3,
					finishDuration:   noTasksWait*2 + 300,
				},
				// parallel task should get picked up as soon as queue frees
				// due to first task finishing
				{
					availableAt:      noTasksWait*3 + 100,
					expectedPoppedAt: noTasksWait*3 + 200,
					finishDuration:   noTasksWait*2 + 400,
				},
				// task queued once second task is finished, causing revert to non parallel max mode
				{
					availableAt:      noTasksWait*4 + 300,
					expectedPoppedAt: noTasksWait*5 + 200,
					finishDuration:   noTasksWait*2 + 200,
				},
			},
			workers:      3,
			emptyDelayAt: []time.Duration{0, noTasksWait, noTasksWait * 2, noTasksWait*4 + 200},
		},
	}

	for testCase, data := range testCases {
		t.Run(testCase, func(t *testing.T) {
			//ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			//defer cancel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			host := "testhost"

			sort.Sort(data.testTasks)
			generatedTestTasks := make([]generatedTestTask, 0, len(data.testTasks))
			for i, tt := range data.testTasks {
				// alternate storage and retrieval
				var task *tasks.Task
				if i%2 == 0 {
					storageTask := tasks.NewStorageTask("t1000", 0, 1000000000, 6152, true, true, strconv.Itoa(i))
					task = tasks.NewTask(nil, storageTask)
				} else {
					retrievalTask := tasks.NewRetrievalTask("t1000", "qmXXXXX", false, strconv.Itoa(i))
					task = tasks.NewTask(retrievalTask, nil)
				}
				task = task.Assign(host, &tasks.InProgress)
				generatedTestTasks = append(generatedTestTasks, generatedTestTask{task, tt})
			}

			tags := []string{}
			stageTimeouts := map[string]time.Duration{}

			// depedencies
			walletAddress := address.TestAddress
			head := &types.TipSet{}
			node := mocks.NewMockFullNode(ctrl)
			ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
			amt := abi.NewTokenAmount(1000000000000)
			node.EXPECT().Version(gomock.AssignableToTypeOf(ctxType)).Return(api.APIVersion{
				Version: "1.2.3",
			}, nil)
			node.EXPECT().WalletBalance(gomock.AssignableToTypeOf(ctxType), gomock.Eq(walletAddress)).
				Return(amt, nil).
				AnyTimes()
			node.EXPECT().ChainHead(gomock.AssignableToTypeOf(ctxType)).Return(head, nil).
				AnyTimes()
			node.EXPECT().StateVerifiedClientStatus(
				gomock.AssignableToTypeOf(ctxType), gomock.Eq(walletAddress), gomock.Eq(head.Key())).
				Return(&amt, nil).
				AnyTimes()
			nodeConfig := tasks.NodeConfig{
				WalletAddress:    walletAddress,
				MinWalletBalance: big.Zero(),
				MinWalletCap:     big.Zero(),
			}
			var closer lotus.NodeCloser = func() {}
			clock := clock.NewMock()
			startTime := clock.Now()
			client := &testAPIClient{
				startTime: startTime,
				clock:     clock,
				tasks:     generatedTestTasks,
				popped:    make(map[string]struct{}),
			}
			receivedTasks := make(chan int, 1)
			taskExecutor := &testTaskExecutor{
				receivedTasks: receivedTasks,
				clock:         clock,
				tasks:         data.testTasks,
			}

			queueIsEmpty := make(chan struct{}, 1)
			e, err := new(ctx, host, stageTimeouts, tags, node, nodeConfig, closer, client, clock, taskExecutor, queueIsEmpty)
			require.NoError(t, err)
			done := make(chan struct{}, 1)
			go func() {
				e.run(ctx, data.workers)
				done <- struct{}{}
			}()

			checkPointsByTime := make(map[time.Duration]*checkPoint)
			for i, task := range data.testTasks {
				_, ok := checkPointsByTime[task.availableAt]
				if !ok {
					checkPointsByTime[task.availableAt] = &checkPoint{time: task.availableAt, tasks: map[int]testTask{}}
				}
				cp, ok := checkPointsByTime[task.expectedPoppedAt]
				if !ok {
					cp = &checkPoint{time: task.expectedPoppedAt, tasks: map[int]testTask{}}
					checkPointsByTime[task.expectedPoppedAt] = cp
				}
				cp.tasks[i] = task
			}
			for _, emptyDelayAt := range data.emptyDelayAt {
				cp, ok := checkPointsByTime[emptyDelayAt]
				if !ok {
					cp = &checkPoint{time: emptyDelayAt, tasks: map[int]testTask{}}
					checkPointsByTime[emptyDelayAt] = cp
				}
				cp.emptyDelayAt = true
			}
			checkPoints := make([]*checkPoint, 0, len(checkPointsByTime))
			for _, cp := range checkPointsByTime {
				checkPoints = append(checkPoints, cp)
			}
			sort.Slice(checkPoints, func(i, j int) bool { return checkPoints[i].time < checkPoints[j].time })
			for _, cp := range checkPoints {
				currentTime := clock.Since(startTime)
				clock.Add(cp.time - currentTime)
				seen := map[int]bool{}
				// check popped tasks
				for i := 0; i < len(cp.tasks); i++ {
					select {
					case poppedTask := <-receivedTasks:
						_, expected := cp.tasks[poppedTask]
						require.True(t, expected)
						require.False(t, seen[poppedTask])
						seen[poppedTask] = true
					case <-ctx.Done():
						t.Fatal("should have popped expected task at given time but didn't")
					}
				}
				// check queue empty
				if cp.emptyDelayAt {
					select {
					case <-queueIsEmpty:
					case <-ctx.Done():
						t.Fatal("expected queue to be empty, but was not")
					}
				}
			}
			e.Close(ctx)
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("did not finish")
			}
		})
	}
}

type checkPoint struct {
	time         time.Duration
	tasks        map[int]testTask
	emptyDelayAt bool
}

type testTask struct {
	availableAt      time.Duration
	expectedPoppedAt time.Duration
	releaseDuration  time.Duration
	finishDuration   time.Duration
}

type generatedTestTask struct {
	*tasks.Task
	testTask
}

type testAPIClient struct {
	startTime time.Time
	clock     clock.Clock
	tasks     []generatedTestTask
	popped    map[string]struct{}
}

func (tapi *testAPIClient) GetTask(ctx context.Context, uuid string) (*tasks.Task, error) {
	for _, task := range tapi.tasks {
		if task.Task.UUID == uuid {
			return task.Task, nil
		}
	}
	return nil, errors.New("not found")
}

func (tapi *testAPIClient) UpdateTask(ctx context.Context, uuid string, r *tasks.UpdateTask) (*tasks.Task, error) {
	for _, task := range tapi.tasks {
		if task.Task.UUID == uuid {
			return task.Task, nil
		}
	}
	return nil, errors.New("not found")
}

func (tapi *testAPIClient) PopTask(ctx context.Context, r *tasks.PopTask) (*tasks.Task, error) {
	for _, task := range tapi.tasks {
		_, popped := tapi.popped[task.UUID]
		if !popped && task.availableAt <= tapi.clock.Since(tapi.startTime) {
			tapi.popped[task.UUID] = struct{}{}
			return task.Task, nil
		}
	}
	return nil, nil
}

func (tapi *testAPIClient) ResetWorker(ctx context.Context, worker string) error {
	return nil
}

type testTaskExecutor struct {
	receivedTasks chan<- int
	clock         clock.Clock
	tasks         testTasks
}

func (tte *testTaskExecutor) runTask(ctx context.Context, index int, releaseWorker func()) error {
	task := tte.tasks[index]
	var timer *clock.Timer
	if task.releaseDuration != time.Duration(0) {
		timer = tte.clock.Timer(task.releaseDuration)
		tte.receivedTasks <- index
		// wait for release
		select {
		case <-timer.C:
			releaseWorker()
		case <-ctx.Done():
			return nil
		}
		// reset to remaining time left on task
		timer.Reset(task.finishDuration - task.releaseDuration)
	} else {
		timer = tte.clock.Timer(task.finishDuration)
		tte.receivedTasks <- index
	}

	// wait for final duration
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
	return nil
}

func (tte *testTaskExecutor) MakeStorageDeal(ctx context.Context, config tasks.NodeConfig, node api.FullNode, task *tasks.StorageTask, updateStage tasks.UpdateStage, log tasks.LogStatus, stageTimeouts map[string]time.Duration, releaseWorker func()) error {
	tag := *(task.Tag)
	index, err := strconv.Atoi(tag)
	if err != nil {
		return err
	}
	if index > len(tte.tasks) {
		return errors.New("index out of bounds")
	}
	return tte.runTask(ctx, index, releaseWorker)
}

func (tte *testTaskExecutor) MakeRetrievalDeal(ctx context.Context, config tasks.NodeConfig, node api.FullNode, task *tasks.RetrievalTask, updateStage tasks.UpdateStage, log tasks.LogStatus, stageTimeouts map[string]time.Duration, releaseWorker func()) error {
	tag := *(task.Tag)
	index, err := strconv.Atoi(tag)
	if err != nil {
		return err
	}
	if index > len(tte.tasks) {
		return errors.New("index out of bounds")
	}
	return tte.runTask(ctx, index, releaseWorker)
}

type testTasks []testTask

func (t testTasks) Len() int {
	return len(t)
}
func (t testTasks) Less(i int, j int) bool {
	if t[i].availableAt == t[j].availableAt {
		return t[i].expectedPoppedAt < t[j].expectedPoppedAt
	}
	return t[i].availableAt < t[j].availableAt
}
func (t testTasks) Swap(i int, j int) {
	t[i], t[j] = t[j], t[i]
}
