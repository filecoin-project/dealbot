package commands

import (
	"context"

	"github.com/filecoin-project/dealbot/testutil"
	"github.com/urfave/cli/v2"
)

// MockTasksCmd generates a series of mock tasks and adds them to the controller
var MockTasksCmd = &cli.Command{
	Name:   "mock_tasks",
	Usage:  "Generate tasks to run on the mock dealbot",
	Flags:  append(EndpointFlags, MockTaskFlags...),
	Action: mockTasksCommand,
}

func mockTasksCommand(cctx *cli.Context) error {
	ctx, cancel := context.WithCancel(ProcessContext())
	defer cancel()

	return testutil.GenerateMockTasks(ctx, cctx)
}
