package tasks

import "github.com/filecoin-project/go-address"

type UpdateStatus func(msg string, keysAndValues ...interface{})

type ClientConfig struct {
	DataDir       string
	NodeDataDir   string
	WalletAddress address.Address
}
