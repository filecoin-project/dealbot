package commands

import (
	"context"
	"time"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/urfave/cli/v2"
)

var DrainCmd = &cli.Command{
	Name:   "drain",
	Usage:  "Stop a dealbot daemon",
	Flags:  append(EndpointFlags, idFlag),
	Action: drainCommand,
}

func drainCommand(c *cli.Context) error {
	host_id := c.String("id")
	controller := client.New(c)

	// drain `host_id`
	if err := controller.Drain(c.Context, host_id); err != nil {
		return err
	}

	// poll until tasks from `host_id` are done.
	for {
		if isActive(c.Context, controller, host_id) {
			time.Sleep(time.Second)
		} else {
			break
		}
	}

	// complete `host_id`
	return controller.Complete(c.Context, host_id)
}

func isActive(ctx context.Context, cli *client.Client, host string) bool {
	tasks, err := cli.ListTasks(ctx)
	if err != nil {
		return false
	}
	for _, t := range tasks {
		if t.WorkedBy != nil && *t.WorkedBy == host {
			if int(t.Status) < 3 {
				return true
			}
		}
	}
	return false
}
