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
		switch t := task.(type) {
		case tasks.StorageTask:
			err = tasks.MakeStorageDeal(cctx.Context, nodeConfig, node, t, logger)
		case tasks.RetrievalTask:
			err = tasks.MakeRetrievalDeal(cctx.Context, nodeConfig, node, t, logger)
		}
		if err != nil {
			log.Error(err)
		}
	}

	return nil
}

func parseTasks() ([]interface{}, error) {
	stdin := bufio.NewReader(os.Stdin)
	dec := json.NewDecoder(stdin)

	var tasksJson []map[string]interface{}
	err := dec.Decode(&tasksJson)
	if err != nil {
		return nil, err
	}

	taskList := make([]interface{}, len(tasksJson))
	for i, taskJson := range tasksJson {
		switch taskJson["Task"] {
		case "retrieval":
			var task tasks.RetrievalTask
			err := (&task).FromMap(taskJson)
			if err != nil {
				return nil, err
			}
			taskList[i] = task
		case "storage":
			var task tasks.StorageTask
			err := (&task).FromMap(taskJson)
			if err != nil {
				return nil, err
			}
			taskList[i] = task
		default:
			return nil, fmt.Errorf("unknown task type: %v", taskJson["Task"])
		}
	}
	return taskList, nil
}
