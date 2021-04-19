package controller

import (
	"os"
	"testing"

	"github.com/filecoin-project/dealbot/controller/sqlitedb"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/google/uuid"
)

const testDBFile = "teststate.db"

func TestLoadTask(t *testing.T) {
	state, err := NewState(sqlitedb.New(testDBFile))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(testDBFile)

	count, err := state.CountTasks()
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("expected 4 tasks, got %d", count)
	}
	t.Log("got", count, "tasks")

	state.saveTask(&tasks.Task{
		UUID:   uuid.New().String()[:8],
		Status: tasks.Available,
		RetrievalTask: &tasks.RetrievalTask{
			Miner:      "t01000",
			PayloadCID: "bafk2bzacedli6qxp43sf54feczjd26jgeyfxv4ucwylujd3xo5s6cohcqbg36",
			CARExport:  false,
		},
	})

	oldCount := count
	count, err = state.CountTasks()
	if err != nil {
		t.Fatal(err)
	}
	if count != oldCount+1 {
		t.Fatalf("expected %d tasks, got %d", oldCount+1, count)
	}
}
