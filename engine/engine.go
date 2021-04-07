package engine

import (
	"context"
	"net/http"
	"time"

	"github.com/filecoin-project/dealbot/config"
	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("engine")

type Engine struct {
	client *client.Client
}

func New(cfg *config.EnvConfig) *Engine {
	workers := 2

	client := client.New(cfg)

	e := &Engine{
		client: client,
	}

	for i := 0; i < workers; i++ {
		go e.worker(i)
	}

	return e
}

func (e *Engine) worker(n int) {
	log.Infow("engine worker started", "worker_id", n)

	for {
		func() {
			time.Sleep(5 * time.Second)

			// fetch tasks
			ctx := context.Background()
			alltasks, err := e.client.ListTasks(ctx)
			if err != nil {
				log.Warnw("list tasks returned error", "err", err)
				return
			}

			// log tasks
			for _, t := range alltasks {
				t.Log(log)
			}

			// pick first available task
			for _, t := range alltasks {
				if t.Status == tasks.Available {
					req := &client.UpdateTaskRequest{
						UUID:     t.UUID,
						Status:   2,
						WorkedBy: "dealbotN", // TODO: add our name
					}

					_, status, err := e.client.UpdateTask(ctx, req)
					if err != nil {
						log.Warnw("update task returned error", "err", err)
						continue
					}
					if status != http.StatusOK {
						log.Warnw("status is different to 200", "status", status)
						continue
					}

					// we have successfully acquired task
					log.Infow("successfully acquired task", "uuid", t.UUID)

					// TODO: process task
					return
				}
			}
		}()
	}
}
