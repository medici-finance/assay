package beta

type store struct {
	n int
}

func (s *store) inc() {
	s.n++
}
