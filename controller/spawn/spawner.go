package spawn

import (
	"errors"

	logging "github.com/ipfs/go-log/v2"
)

var (
	log            = logging.Logger("spawner")
	RegionNotFound = errors.New("region not found")
	DaemonNotFound = errors.New("daemon not found")
)

type Daemon struct {
	Id         string   `json:"id,omitempty"`
	Region     string   `json:"region"`
	Tags       []string `json:"tags" default:"[]"`
	Workers    int      `json:"workers" default:"1"`
	MinFil     int      `json:"minfil" default:"0"`
	MinCap     int      `json:"mincap" default:"0"`
	DockerRepo string   `json:"dockerrepo,omitempty"`
	DockerTag  string   `json:"dockerrtag,omitempty"`
	Wallet     *Wallet  `json:"wallet,omitempty"`
}

type Wallet struct {
	Address  string `json:"address,omitempty"`
	Exported string `json:"exported,omitempty"`
}

type Spawner interface {
	Spawn(*Daemon) error
	Get(string, string) (*Daemon, error)
	List(string) ([]*Daemon, error)
	Regions() []string
}
