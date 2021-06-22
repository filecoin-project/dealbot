package spawn

type LocalSpawner struct{}

func NewLocal() (s *LocalSpawner) {
	s = &LocalSpawner{}
	return s
}
