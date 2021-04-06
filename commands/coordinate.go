package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/filecoin-project/dealbot/config"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"
)

var CoordinateCmd = &cli.Command{
	Name:   "coordinate",
	Usage:  "run multiple deals configured by a json input on stdin",
	Action: coordinateCommand,
	Flags:  DealFlags,
}

func coordinateCommand(cctx *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	taskList, err := parseTasks()
	if err != nil {
		return err
	}

	nodeConfig, node, closer, err := setupCLIClient(cctx)
	if err != nil {
		return err
	}
	defer closer()

	logger := func(msg string, keysAndValues ...interface{}) {
		log.Infow(msg, keysAndValues...)
	}

	for _, task := range taskList {
		var err error
		if task.StorageTask != nil {
			err = tasks.MakeStorageDeal(cctx.Context, nodeConfig, node, *task.StorageTask, logger)
		} else if task.RetrievalTask != nil {
			err = tasks.MakeRetrievalDeal(cctx.Context, nodeConfig, node, *task.RetrievalTask, logger)
		}
		if err != nil {
			log.Error(err)
		}
	}

	return nil
}

func parseTasks() ([]tasks.Task, error) {
	stdin := bufio.NewReader(os.Stdin)
	dec := json.NewDecoder(stdin)

	var taskList []tasks.Task
	err := dec.Decode(&taskList)
	if err != nil {
		return nil, err
	}

	// validate we only have one retrieval or storage per task
	for _, task := range taskList {

		if task.StorageTask == nil && task.RetrievalTask == nil {
			return nil, fmt.Errorf("task %v must have exactly one storage or retrieval task", task.UUID)
		}
		if task.StorageTask != nil && task.RetrievalTask != nil {
			return nil, fmt.Errorf("task %v should not have both a storage and retrieval task", task.UUID)
		}
	}

	return taskList, nil
}
