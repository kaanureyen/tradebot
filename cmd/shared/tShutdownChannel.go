package shared

type ShutdownChannel struct {
	send []chan struct{}
	recv []chan struct{}
	Done chan struct{}
}

func (s *ShutdownChannel) Init() {
	s.Done = make(chan struct{})
}

func (s *ShutdownChannel) Get() (chan struct{}, chan struct{}) {
	send := make(chan struct{})
	recv := make(chan struct{})
	s.send = append(s.send, send)
	s.recv = append(s.recv, recv)
	return send, recv
}

func (s *ShutdownChannel) Shutdown() {
	go func() {
		// receive done signals
		for i := range s.recv {
			<-(s.recv)[i]
		}

		// close send channels
		for i := range s.send {
			close((s.send)[i])
		}

		// set done
		s.Done <- struct{}{}
	}()

	// send stop signals
	for i := range s.send {
		(s.send)[i] <- struct{}{}
	}
}
