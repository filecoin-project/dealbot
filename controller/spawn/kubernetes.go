package spawn

type KubernetesSpawner struct{}

func (s *KubernetesSpawner) Spawn(d *Daemon) error {
	return nil
}

func (s *KubernetesSpawner) Get(regionid string, daemonid string) *Daemon {
	return nil
}

func (s *KubernetesSpawner) List(region string) []*Daemon {
	return []*Daemon{}
}

func (s *KubernetesSpawner) Regions() []string {
	return []string{}
}

func NewKubernetes(regions []string) (s *KubernetesSpawner) {
	s = &KubernetesSpawner{}
	return s
}
