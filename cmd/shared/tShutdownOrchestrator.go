package shared

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

type ShutdownOrchestrator struct {
	send []chan struct{}
	recv []chan struct{}
	Done chan struct{}
}

func (s *ShutdownOrchestrator) Start() {
	osCloseSignal := make(chan os.Signal, 1)
	signal.Notify(osCloseSignal, syscall.SIGINT, syscall.SIGTERM)

	s.Done = make(chan struct{})

	go func() {
		defer close(osCloseSignal)
		sig := <-osCloseSignal
		log.Println("Received int/term signal, will quit:", sig)
		s.Shutdown()
	}()
}

func (s *ShutdownOrchestrator) Get() (chan struct{}, chan struct{}) {
	send := make(chan struct{})
	recv := make(chan struct{})
	s.send = append(s.send, send)
	s.recv = append(s.recv, recv)
	return send, recv
}

func (s *ShutdownOrchestrator) Shutdown() {
	go func() {
		// receive done signals
		for i := range s.recv {
			<-(s.recv)[i]
		}

		// close send channels
		for i := range s.send {
			close((s.send)[i])
		}

		// set done for every optional receiver
		go func() {
			for {
				s.Done <- struct{}{}
			}
		}()
	}()

	// send stop signals
	for i := range s.send {
		(s.send)[i] <- struct{}{}
	}
}
