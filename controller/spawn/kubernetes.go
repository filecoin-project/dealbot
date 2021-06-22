package spawn

type KubernetesSpawner struct{}

func NewKubernetes(regions []string) (s *KubernetesSpawner) {
	s = &KubernetesSpawner{}
	return s
}
