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
	var chPeriodicStats shared.SlicePeriodicStats
	chPeriodicStats.Add(shared.RedisChannel, time.Minute, &shutdownOrchestrator)
	chPeriodicStats.Add(shared.RedisChannel, time.Hour, &shutdownOrchestrator)
	chPeriodicStats.Add(shared.RedisChannel, time.Hour*24, &shutdownOrchestrator)
	stats := chPeriodicStats.FanIn(&shutdownOrchestrator)

	func() {
		for v := range stats {
			log.Println("Stat for", v.Period, "is", v.Value)
		}
	}()

	<-shutdownOrchestrator.Done
	log.Println("Exiting...")
}
