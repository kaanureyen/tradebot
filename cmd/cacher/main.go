package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared/constants"
	"github.com/redis/go-redis/v9"
)

var rdb = redis.NewClient(&redis.Options{
	Addr: constants.RedisAddress,
})
var ctx = context.Background()

func main() {
	// show line number in logs, show microseconds, add prefix
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix("[cacher] ")
	log.Println("Started")

	// Create a channel to listen for signals from the OS for graceful shutdown
	shutdownSignals := make(chan os.Signal, 1)
	defer close(shutdownSignals)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	var shutdownChannels constants.ShutdownChannel

	go func() {
		defer shutdownChannels.CloseAll()
		sig := <-shutdownSignals
		log.Println("Received int/term signal, will quit:", sig)
		shutdownChannels.SendAll()
	}()

	chMinute := periodicPriceStats(constants.RedisChannel, shutdownChannels.Get(), time.Minute)
	chHour := periodicPriceStats(constants.RedisChannel, shutdownChannels.Get(), time.Hour)
	chDay := periodicPriceStats(constants.RedisChannel, shutdownChannels.Get(), 24*time.Hour)

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

func periodicPriceStats(subCh string, stopCh chan struct{}, period time.Duration) chan constants.AggregatedTradeInfo {
	return calculateMinMaxLast(
		unmarshalTradeDatePrice(
			subscribe(
				subCh,
				stopCh,
			),
		),
		time.Now(),
		period,
	)
}

// calculates and sends MinMaxLast-s from TradeDatePrice-s from a start date per each resolution
func calculateMinMaxLast(chDatePrice chan constants.TradeDatePrice, startDate time.Time, resolution time.Duration) chan constants.AggregatedTradeInfo {
	lastSentDate := startDate
	var curMinMaxLast constants.AggregatedTradeInfo
	curMinMaxLast.SetDefault()

	out := make(chan constants.AggregatedTradeInfo)
	go func() {
		defer close(out)
		for v := range chDatePrice {
			// parse price to float
			p, err := strconv.ParseFloat(v.Price, 64)
			if err != nil {
				log.Println("Error while parsing price as float: ", err)
				continue
			}

			d := time.UnixMilli(v.TradeDate)
			delta := d.Sub(lastSentDate)
			if delta >= resolution {
				if !curMinMaxLast.IsDefault() {
					out <- curMinMaxLast
				}
				curMinMaxLast.SetDefault()
				lastSentDate = lastSentDate.Add(resolution)
			}
			if delta >= 0 {
				curMinMaxLast.Update(d, p)
			} else {
				log.Println("[Info] Discarding data:", v, "due to having a timestamp before the last processed interval:", lastSentDate)
			}
		}
	}()
	return out
}

func unmarshalTradeDatePrice(inp chan string) chan constants.TradeDatePrice {
	out := make(chan constants.TradeDatePrice)
	go func() {
		defer close(out)
		for msg := range inp {
			var msgStruct constants.TradeDatePrice
			err := json.Unmarshal([]byte(msg), &msgStruct)
			if err != nil {
				log.Println("Failed to unmarshal to constants.TradeDatePrice:", err)
				continue
			}
			out <- msgStruct
		}
	}()
	return out
}

func subscribe(subCh string, done chan struct{}) chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		for {
			ch := rdb.Subscribe(ctx, subCh).Channel()

			for {
				select {
				case v, ok := <-ch:
					if !ok {
						break
					}
					out <- v.Payload

				case <-done:
					log.Println("Stopping subscription:", subCh)
					return
				}
			}
		}
	}()
	return out
}
