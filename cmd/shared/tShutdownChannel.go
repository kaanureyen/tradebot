package shared

type ShutdownChannel []chan struct{}

func (s *ShutdownChannel) Get() chan struct{} {
	ch := make(chan struct{})
	*s = append(*s, ch)
	return ch
}

func (s *ShutdownChannel) SendAll() {
	for i := range *s {
		(*s)[i] <- struct{}{}
	}
}

func (s *ShutdownChannel) CloseAll() {
	for i := range *s {
		close((*s)[i])
	}
}
