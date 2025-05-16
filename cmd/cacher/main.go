package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared/constants"
	"github.com/redis/go-redis/v9"
)

var rdb = redis.NewClient(&redis.Options{
	Addr: constants.RedisAddress,
})
var ctx = context.Background()
var shutdownChannels constants.ShutdownChannel

func main() {
	// show line number in logs, show microseconds, add prefix
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix("[cacher] ")
	log.Println("Started")

	// Create a channel to listen for signals from the OS for graceful shutdown
	shutdownSignals := make(chan os.Signal, 1)
	defer close(shutdownSignals)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		defer shutdownChannels.CloseAll()
		sig := <-shutdownSignals
		log.Println("Received int/term signal, will quit:", sig)
		shutdownChannels.SendAll()
	}()

	chMinute := periodicPriceStats(constants.RedisChannel, time.Minute)
	chHour := periodicPriceStats(constants.RedisChannel, time.Hour)
	chDay := periodicPriceStats(constants.RedisChannel, 24*time.Hour)

	for {
		if chMinute == nil && chHour == nil && chDay == nil {
			break
		}

		select {
		case v, ok := <-chMinute:
			if !ok {
				chMinute = nil
				continue
			}
			log.Printf("%#v\n", v)

		case v, ok := <-chHour:
			if !ok {
				chHour = nil
				continue
			}
			log.Println(v)

		case v, ok := <-chDay:
			if !ok {
				chDay = nil
				continue
			}
			log.Println(v)
		}
	}

	log.Println("Exiting...")
}
