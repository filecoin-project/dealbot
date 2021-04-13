package main

import (
	"os"
	"testing"

	"github.com/filecoin-project/dealbot/devnet"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"devnet": func() int {
			devnet.Main()
			return 0
		},
	}))
}

func TestStorageAndRetrievalTasks(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/scripts",
	})
}
