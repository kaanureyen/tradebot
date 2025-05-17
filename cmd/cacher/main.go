package main

import (
	"log"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared"
)

func main() {
	shared.Logger("[cacher] ")

	// start shutdown orchestrator
	var shutdownOrchestrator shared.ShutdownOrchestrator
	shutdownOrchestrator.Start()

	// start stat aggregators for different time periods & fan in into stats
	var chPeriodicStats SlicePeriodicStats
	chPeriodicStats.Add(shared.RedisChannel, 5*time.Second, &shutdownOrchestrator)
	chPeriodicStats.Add(shared.RedisChannel, time.Hour, &shutdownOrchestrator)
	chPeriodicStats.Add(shared.RedisChannel, time.Hour*24, &shutdownOrchestrator)
	stats := chPeriodicStats.FanIn(&shutdownOrchestrator)

	func() {
		for v := range stats {
			log.Printf("Stat for %v is %#v", v.period, v.value.lastTime.UTC())
		}
	}()

	<-shutdownOrchestrator.Done
	log.Println("Exiting...")
}
