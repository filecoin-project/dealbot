package spawn

import (
	"math/rand"

	"github.com/filecoin-project/go-state-types/big"
)

// This is a test that the values file base.yaml is setup correctly for the chart.
// It's possible that the values file is proper yaml and would still not be valid
// for the chart, so hopefully this will detect certain issues.

func randomString(n int) string {
	abc := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = abc[rand.Intn(len(abc))]
	}
	return string(b)
}

func randomDaemon() *Daemon {
	// random id and region
	return &Daemon{
		Id:         randomString(5),
		Region:     randomString(5),
		Tags:       []string{randomString(5)},
		Workers:    rand.Intn(10),
		MinFil:     big.NewIntUnsigned(rand.Uint64()),
		MinCap:     big.NewIntUnsigned(rand.Uint64()),
		DockerRepo: randomString(5),
		DockerTag:  randomString(5),
		Wallet: &Wallet{
			Address:  randomString(5),
			Exported: randomString(5),
		},
	}
}
