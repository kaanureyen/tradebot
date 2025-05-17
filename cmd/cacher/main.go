package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared"
)

var shutdownChannels shared.ShutdownChannel

func main() {
	// show line number in logs, show microseconds, add prefix
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix("[cacher] ")
	log.Println("Started")

	// Create a channel to listen for signals from the OS for graceful shutdown
	shutdownSignals := make(chan os.Signal, 1)
	defer close(shutdownSignals)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	// initialize shutdown orchestrator
	shutdownChannels.Init()
	go func() {
		sig := <-shutdownSignals
		log.Println("Received int/term signal, will quit:", sig)
		shutdownChannels.SendAll()
	}()

	var chPeriodicStats shared.SlicePeriodicStats
	chPeriodicStats.Add(shared.RedisChannel, time.Minute, &shutdownChannels)
	chPeriodicStats.Add(shared.RedisChannel, time.Hour, &shutdownChannels)
	chPeriodicStats.Add(shared.RedisChannel, time.Hour*24, &shutdownChannels)

	stats := make(chan shared.PeriodicStats)
	stopCh, finishedCh := chPeriodicStats.FanIn(stats)
	for i := range stopCh {
		stopCh[i], finishedCh[i] = shutdownChannels.Get()
	}

	go func() {
		<-shutdownChannels.Done
		log.Println("Exiting stats channel")
		close(stats)
	}()

	func() {
		for v := range stats {
			log.Println("Stat for", v.Period, "is", v.Value)
		}
	}()

	log.Println("Exiting...")
}
