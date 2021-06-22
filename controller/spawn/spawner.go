package spawn

type Daemon struct {
	Id     string   `json:"id,omitempty"`
	Region string   `json:"region"`
	Tags   []string `json:"tags"`
}

type Spawner interface {
	Spawn(*Daemon) error
	Get(string, string) *Daemon
	List(string) []*Daemon
	Regions() []string
}
