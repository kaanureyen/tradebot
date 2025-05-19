package main

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared"
	"github.com/redis/go-redis/v9"
)

func PeriodicPriceStats(subCh string, period time.Duration, shutdownOrchestrator *shared.ShutdownOrchestrator) chan AggregatedTradeInfo {
	stop, finished := shutdownOrchestrator.Get()
	return calculatePriceStats(
		unmarshalTradeDatePrice(
			subscribeRedis(
				subCh,
				stop,
			),
		),
		time.Now().Truncate(24*time.Hour),
		period,
		finished,
	)
}

// calculates and sends AggregateTradeInfo-s from TradeDatePrice-s from a start date per each resolution
func calculatePriceStats(chDatePrice chan shared.TradeDatePrice, startDate time.Time, resolution time.Duration, finished chan struct{}) chan AggregatedTradeInfo {
	lastSentDate := startDate

	var curAgg AggregatedTradeInfo
	curAgg.SetDefault()

	out := make(chan AggregatedTradeInfo)
	go func() {
		defer func() {
			close(out)
			finished <- struct{}{}
		}()

		for v := range chDatePrice {
			// parse price to float
			p, err := strconv.ParseFloat(v.Price, 64)
			if err != nil {
				log.Println("[Warning] while parsing price as float. Skipping the data. Error:: ", err)
				continue
			}

			// determine its time-group
			d := time.UnixMilli(v.TradeDate)
			delta := d.Sub(lastSentDate)
			if delta >= resolution { // latest received message belongs to the next group
				if !curAgg.IsDefault() { // send current aggregation if populated
					out <- curAgg
				}
				curAgg.SetDefault() // reset for the next time group

				lastSentDate = lastSentDate.Add((delta / resolution) * resolution) // move the time group marker forward

			}
			if delta >= 0 {
				curAgg.Update(d, p)
			} else {
				log.Println("[Warning] Discarding data:", v, "due to having a timestamp before the last processed interval:", lastSentDate)
			}
		}
	}()
	return out
}

func unmarshalTradeDatePrice(inp chan string) chan shared.TradeDatePrice {
	out := make(chan shared.TradeDatePrice)
	go func() {
		defer close(out)
		for msg := range inp {
			var msgStruct shared.TradeDatePrice
			err := json.Unmarshal([]byte(msg), &msgStruct)
			if err != nil {
				log.Println("[Warning] Failed to unmarshal to TradeDatePrice. Skipping the data. Error::", err)
				continue
			}
			out <- msgStruct
		}
	}()
	return out
}

// accepts redis channel name to connect. returns redis message receive channel
func subscribeRedis(subCh string, done chan struct{}) chan string {
	var rdb = redis.NewClient(&redis.Options{
		Addr: shared.RedisAddress,
	})
	var ctx = context.Background()
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
					log.Println("[Info] Stopping subscription:", subCh)
					return
				}
			}
		}
	}()
	return out
}
