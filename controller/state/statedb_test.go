package state

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/dealbot/tasks"
	"github.com/google/uuid"
)

func TestLoadTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := ioutil.TempDir("", "testdealbot")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	stateInterface, err := NewStateDB(ctx, "sqlite", filepath.Join(tmpDir, "teststate.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	state := stateInterface.(*stateDB)

	count, err := state.countTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 tasks, got %d", count)
	}
	t.Log("got", count, "tasks")

	state.saveTask(ctx, &tasks.AuthenticatedTask{
		Task: tasks.Task{
			UUID:   uuid.New().String()[:8],
			Status: tasks.Available,
			RetrievalTask: &tasks.RetrievalTask{
				Miner:      "t01000",
				PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
				CARExport:  false,
			},
		},
		Signature: []byte{},
	})

	oldCount := count
	count, err = state.countTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != oldCount+1 {
		t.Fatalf("expected %d tasks, got %d", oldCount+1, count)
	}
}
