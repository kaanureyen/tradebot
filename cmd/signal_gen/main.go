package main

import (
	"log"

	"github.com/kaanureyen/tradebot/cmd/shared"
)

func main() {
	shutdownOrchestrator := shared.InitCommon("signal_gen") // set logger name, start http health endpoint, initialize & start shutdownOrchestrator
	defer func() {
		<-shutdownOrchestrator.Done // blocks until every shutdownOrchestrator.Get()'s recv is sent an empty struct, after a interrupt/terminate signal.
		log.Println("Exiting...")
	}()
}
