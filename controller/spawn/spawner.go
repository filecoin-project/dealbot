package spawn

import (
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("spawner")

type Daemon struct {
	Id      string   `json:"id,omitempty"`
	Region  string   `json:"region"`
	Tags    []string `json:"tags" default:"[]"`
	Wallet  string   `json:"wallet" default:""`
	Workers int      `json:"workers" default:"1"`
	MinFil  int      `json:"minfil" default:"0"`
	MinCap  int      `json:"mincap" default:"0"`
}

type Spawner interface {
	Spawn(*Daemon) error
	Get(string, string) *Daemon
	List(string) []*Daemon
	Regions() []string
}
