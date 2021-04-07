package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/filecoin-project/dealbot/config"
	"github.com/filecoin-project/dealbot/controller"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"
)

var ClientCmd = &cli.Command{
	Name:   "client",
	Usage:  "retrieve deal tasks from a controller and run them locally",
	Action: clientCommand,
	Flags: append(DealFlags, []cli.Flag{
		&cli.StringFlag{
			Name:     "controller-url",
			Usage:    "http url to the controller that will be serving tasks to this client",
			Required: true,
		},
	}...),
}

func clientCommand(cctx *cli.Context) error {
	cfg := &config.EnvConfig{}
	if err := cfg.Load(); err != nil {
		return err
	}

	// request tasks
	controllerURL := cctx.String("controller-url")
	resp, err := http.Get(controllerURL + "tasks")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	taskList, err := parseTasks(resp.Body)
	if err != nil {
		return err
	}

	nodeConfig, node, closer, err := setupCLIClient(cctx)
	if err != nil {
		return err
	}
	defer closer()

	for _, task := range taskList {
		logger := func(msg string, keysAndValues ...interface{}) {
			sendStatus(controllerURL, task, msg, keysAndValues)
		}

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

func parseTasks(r io.Reader) ([]tasks.Task, error) {
	dec := json.NewDecoder(r)

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

func sendStatus(controllerURL string, task tasks.Task, msg string, keysAndValues ...interface{}) {
	log.Infow(msg, keysAndValues...)

	status := controller.TaskStatus{
		UUID:    task.UUID,
		Type:    msg,
		Payload: keysAndValues,
	}

	j, err := json.Marshal(&status)
	if err != nil {
		log.Error(err)
		return
	}

	_, err = http.Post(controllerURL+"status", "application/json", bytes.NewReader(j))
	if err != nil {
		log.Error(err)
		return
	}
}
