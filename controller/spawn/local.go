package spawn

import "fmt"

type LocalSpawner struct {
	list []*Daemon
}

func (s *LocalSpawner) Spawn(d *Daemon) error {
	fmt.Println("adding", d.Id)
	s.list = append(s.list, d)
	return nil
}

func (s *LocalSpawner) Get(regionid string, daemonid string) *Daemon {
	return s.list[0]
}

func (s *LocalSpawner) List(regionid string) []*Daemon {
	fmt.Println("listing", s.list)
	return s.list
}

func (s *LocalSpawner) Regions() []string {
	fmt.Println("getting region (local)")
	return []string{"local"}
}

func NewLocal() (s *LocalSpawner) {
	s = &LocalSpawner{
		list: make([]*Daemon, 0),
	}
	return s
}
