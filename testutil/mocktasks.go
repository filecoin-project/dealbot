package testutil

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"

	"github.com/filecoin-project/dealbot/controller/client"
	"github.com/filecoin-project/dealbot/tasks"
	"github.com/urfave/cli/v2"
)

// GenerateMockTasks generates mock tasks for the mockbot -- this will not
// not work with a real dealbot
func GenerateMockTasks(ctx context.Context, cliCtx *cli.Context) error {
	client := client.New(cliCtx)
	numTasks := cliCtx.Int("count")
	numRetrievals := cliCtx.Int("retrievals")
	if numRetrievals > numTasks {
		return fmt.Errorf("Cannot have more retrievals than total tasks")
	}
	for i := 0; i < numRetrievals; i++ {
		rt := tasks.NewRetrievalTask(generateRandomMiner(), generateRandomCID(), false, "")
		_, err := client.CreateRetrievalTask(ctx, rt)
		if err != nil {
			return err
		}
	}
	for i := numRetrievals; i < numTasks; i++ {
		st := tasks.NewStorageTask(generateRandomMiner(), int64(rand.Uint64()), int64(rand.Uint64()), int64(rand.Uint64()), rand.Intn(2) != 0, rand.Intn(2) != 0, "")

		_, err := client.CreateStorageTask(ctx, st)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateRandomMiner() string {
	minerNumer := rand.Intn(10)
	return fmt.Sprintf("f%d", minerNumer)
}

func generateRandomCID() string {
	buf := make([]byte, 100)
	_, _ = rand.Read(buf)
	return base64.StdEncoding.EncodeToString(buf)
}
