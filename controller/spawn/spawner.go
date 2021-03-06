package spawn

import (
	"errors"

	"github.com/filecoin-project/go-state-types/big"
	logging "github.com/ipfs/go-log/v2"
)

var (
	log            = logging.Logger("spawner")
	RegionNotFound = errors.New("region not found")
	DaemonNotFound = errors.New("daemon not found")
)

type Daemon struct {
	Id               string   `json:"id,omitempty"`
	Region           string   `json:"region,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	Workers          int      `json:"workers,omitempty"`
	MinFil           big.Int  `json:"minfil,omitempty"`
	MinCap           big.Int  `json:"mincap,omitempty"`
	DockerRepo       string   `json:"dockerrepo,omitempty"`
	DockerTag        string   `json:"dockerrtag,omitempty"`
	HelmChartVersion string   `json:"helmchartversion,omitempty"`
	HelmChartRepoUrl string   `json:"helmchartrepourl,omitempty"`
	LotusDockerRepo  string   `json:"lotusdockerrepo,omitempty"`
	LotusDockerTag   string   `json:"lotusdockertag,omitempty"`
	Wallet           *Wallet  `json:"wallet,omitempty"`
}

type Wallet struct {
	Address  string `json:"address,omitempty"`
	Exported string `json:"exported,omitempty"`
}

type Spawner interface {
	Spawn(*Daemon) error
	Get(string, string) (*Daemon, error)
	Shutdown(string, string) error
	List(string) ([]*Daemon, error)
	Regions() []string
}
