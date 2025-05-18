package main

import (
	"log"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared"
)

func main() {
	shutdownOrchestrator := shared.InitCommon("cacher") // set logger name, start http health endpoint, initialize & start shutdownOrchestrator
	defer func() {
		<-shutdownOrchestrator.Done // blocks until every shutdownOrchestrator.Get()'s recv is sent an empty struct, after a interrupt/terminate signal.
		log.Println("[Info] Exiting...")
	}()

	// start stat aggregators for different time periods & fan in into stats
	var chPeriodicStats SlicePeriodicStats
	chPeriodicStats.Add(shared.RedisChannel, 5*time.Second, shutdownOrchestrator)
	chPeriodicStats.Add(shared.RedisChannel, time.Hour, shutdownOrchestrator)
	chPeriodicStats.Add(shared.RedisChannel, time.Hour*24, shutdownOrchestrator)
	stats := chPeriodicStats.FanIn(shutdownOrchestrator)

	func() {
		for v := range stats {
			log.Printf("[Info] Stat for %v is %#v", v.period, v.value.lastTime.UTC())
		}
	}()
}
