package state

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p-core/crypto"
)

const jsonTestDeals = "../../devnet/sample_tasks.json"

func TestLoadTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := ioutil.TempDir("", "testdealbot")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), nil, nil)
	if err != nil {
		t.Fatalf("could not create state DB: %s", err)
	}
	state, ok := stateInterface.(*stateDB)
	if !ok {
		t.Fatal("returned wrong type")
	}

	count, err := state.countTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}

	err = state.saveTask(ctx, &tasks.Task{
		UUID:   uuid.New().String()[:8],
		Status: tasks.Available,
		RetrievalTask: &tasks.RetrievalTask{
			Miner:      "t01000",
			PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
			CARExport:  false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	oldCount := count
	count, err = state.countTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != oldCount+1 {
		t.Fatalf("expected %d tasks, got %d", oldCount+1, count)
	}
}

func TestAssignTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := ioutil.TempDir("", "testdealbot")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	key, err := makeKey()
	if err != nil {
		t.Fatal(err)
	}

	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), key, nil)
	if err != nil {
		t.Fatalf("could not create state DB: %s", err)
	}
	state, ok := stateInterface.(*stateDB)
	if !ok {
		t.Fatal("returned wrong type")
	}

	err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	if err != nil {
		t.Fatal(err)
	}

	task := &tasks.Task{
		UUID:   uuid.New().String()[:8],
		Status: tasks.Available,
		RetrievalTask: &tasks.RetrievalTask{
			Miner:      "t01000",
			PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
			CARExport:  false,
		},
	}
	err = state.saveTask(ctx, task)
	if err != nil {
		t.Fatalf("failed to save task: %s", err)
	}

	taskCount, err := state.countTasks(ctx)
	if err != nil {
		t.Fatalf("failed to count tasks: %s", err)
	}

	seen := make(map[string]struct{}, taskCount)
	for i := 0; i < taskCount; i++ {
		worker := fmt.Sprintf("tester-%d", i)
		req := client.UpdateTaskRequest{
			Status:   tasks.InProgress,
			WorkedBy: worker,
		}
		task, err = state.AssignTask(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if task == nil {
			t.Fatal("Did not find task to assign")
		}
		if task.WorkedBy != worker {
			t.Fatalf("expected task.WorkedBy to be %q, got %q", worker, task.WorkedBy)
		}
		_, found := seen[task.UUID]
		if found {
			t.Fatal("Assigned task that was previously assigned")
		}

		history, err := state.TaskHistory(ctx, task.UUID)
		if err != nil {
			t.Fatal(err)
		}

		if len(history) != 2 {
			t.Fatalf("expected 2 task events, got %d", len(history))
		}
		if history[0].Status != tasks.Available {
			t.Error("wrong status for 1st history")
		}
		if history[1].Status != tasks.InProgress {
			t.Error("wrong status for 2nd history")
		}

		seen[task.UUID] = struct{}{}
	}

	task, err = state.AssignTask(ctx, client.UpdateTaskRequest{
		Status:   tasks.InProgress,
		WorkedBy: "it's me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if task != nil {
		t.Fatal("Shoud not be able to assign more tasks")
	}
}

func TestAssignConcurrentTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := ioutil.TempDir("", "testdealbot")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	key, err := makeKey()
	if err != nil {
		t.Fatal(err)
	}

	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), key, nil)
	if err != nil {
		t.Fatalf("could not create state DB: %s", err)
	}
	state, ok := stateInterface.(*stateDB)
	if !ok {
		t.Fatal("returned wrong type")
	}

	err = populateTestTasks(ctx, jsonTestDeals, stateInterface)
	if err != nil {
		t.Fatal(err)
	}

	taskCount, err := state.countTasks(ctx)
	if err != nil {
		t.Fatalf("failed to count tasks: %s", err)
	}

	release := make(chan struct{})
	assigned := make([]*tasks.Task, taskCount)
	errChan := make(chan error)
	t.Log("concurrently assigning", taskCount, "tasks")
	for i := 0; i < taskCount; i++ {
		go func(n int) {
			worker := fmt.Sprintf("worker-%d", n)
			<-release
			req := client.UpdateTaskRequest{
				Status:   tasks.InProgress,
				WorkedBy: worker,
			}

			task, err := state.AssignTask(ctx, req)
			if err != nil {
				errChan <- err
				return
			}
			assigned[n] = task
			errChan <- nil
		}(i)
	}

	close(release)
	for i := 0; i < taskCount; i++ {
		err = <-errChan
		if err != nil {
			t.Error(err)
		}
	}

	for i := 0; i < taskCount; i++ {
		task := assigned[i]
		if task == nil {
			t.Log("did not find task to assign")
			continue
		}
		history, err := state.TaskHistory(ctx, task.UUID)
		if err != nil {
			t.Fatal(err)
		}

		if len(history) != 2 {
			t.Errorf("expected 2 task events, got %d", len(history))
		}
		if history[0].Status != tasks.Available {
			t.Error("wrong status for 1st history")
		}
		if history[1].Status != tasks.InProgress {
			t.Error("wrong status for 2nd history")
		}
	}
}

func populateTestTasks(ctx context.Context, jsonTests string, state State) error {
	sampleTaskFile, err := os.Open(jsonTestDeals)
	if err != nil {
		return err
	}
	defer sampleTaskFile.Close()
	sampleTasks, err := ioutil.ReadAll(sampleTaskFile)
	if err != nil {
		return err
	}
	byTask := make([]json.RawMessage, 0)
	if err = json.Unmarshal(sampleTasks, &byTask); err != nil {
		return err
	}
	for _, task := range byTask {
		rt := tasks.RetrievalTask{}
		if err = json.Unmarshal(task, &rt); err != nil {
			st := tasks.StorageTask{}
			if err = json.Unmarshal(task, &st); err != nil {
				return fmt.Errorf("could not decode sample task as either storage or retrieval %s: %w", task, err)
			}
			if _, err = state.NewStorageTask(ctx, &st); err != nil {
				return err
			}
		} else {
			if _, err = state.NewRetrievalTask(ctx, &rt); err != nil {
				return err
			}
		}
	}
	return nil
}

func makeKey() (crypto.PrivKey, error) {
	const identity = ".dealbot.test.key"

	// make a new identity
	pr, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		return nil, err
	}
	return pr, nil
}
