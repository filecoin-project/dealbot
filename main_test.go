package main

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/filecoin-project/dealbot/devnet"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"devnet": func() int {
			devnet.Main()
			return 0
		},
		"dealbot": main1,
	}))
}

func TestStorageAndRetrievalTasks(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/scripts",
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"readenv": func(ts *testscript.TestScript, neg bool, args []string) {
				value := ts.ReadFile(args[1])
				ts.Setenv(args[0], value)
				return
			},
			"sleep": func(ts *testscript.TestScript, neg bool, args []string) {
				secs, err := strconv.Atoi(args[0])
				if err != nil {
					panic(err)
				}
				time.Sleep(time.Duration(secs) * time.Second)
				return
			},
		},
	})
}
