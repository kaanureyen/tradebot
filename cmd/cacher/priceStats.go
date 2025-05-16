package main

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared/constants"
)

func periodicPriceStats(subCh string, period time.Duration) chan constants.AggregatedTradeInfo {
	return calculatePriceStats(
		unmarshalTradeDatePrice(
			subscribeRedis(
				subCh,
				shutdownChannels.Get(),
			),
		),
		time.Now(),
		period,
	)
}

// calculates and sends MinMaxLast-s from TradeDatePrice-s from a start date per each resolution
func calculatePriceStats(chDatePrice chan constants.TradeDatePrice, startDate time.Time, resolution time.Duration) chan constants.AggregatedTradeInfo {
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

func subscribeRedis(subCh string, done chan struct{}) chan string {
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
