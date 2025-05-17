package shared

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type PeriodicStats struct {
	ch      chan AggregatedTradeInfo // in
	Period  time.Duration            // in (param)
	RedisCh string                   // in (param)
	Value   AggregatedTradeInfo      // out
}

type SlicePeriodicStats []PeriodicStats

func (s *SlicePeriodicStats) Add(subCh string, period time.Duration, shutdownChannel *ShutdownOrchestrator) {
	stop, finished := shutdownChannel.Get()
	*s = append(*s, PeriodicStats{periodicPriceStats(subCh, period, stop, finished), period, subCh, AggregatedTradeInfo{}})
}

func (s *SlicePeriodicStats) FanIn(shutdownOrchestrator *ShutdownOrchestrator) chan PeriodicStats {
	stats := make(chan PeriodicStats)
	go func() {
		<-shutdownOrchestrator.Done
		log.Println("Closing stats channel")
		close(stats)
		log.Println("Closed stats channel")
	}()

	sigStop := make([]chan struct{}, len(*s))
	sigDone := make([]chan struct{}, len(*s))
	for i := range len(*s) {
		sigStop[i], sigDone[i] = shutdownOrchestrator.Get()
	}
	for i, v := range *s {
		go func() {
			for {
				select {
				case val, ok := <-v.ch:
					if !ok {
						(*s)[i].ch = nil
						continue
					}
					v.Value = val
					stats <- v

				case <-sigStop[i]:
					log.Println("Stopping PeriodicStats FanIn with period:", v.Period)
					sigDone[i] <- struct{}{}
					return
				}
			}
		}()
	}
	return stats
}

func periodicPriceStats(subCh string, period time.Duration, stop chan struct{}, finished chan struct{}) chan AggregatedTradeInfo {
	return calculatePriceStats(
		unmarshalTradeDatePrice(
			subscribeRedis(
				subCh,
				stop,
			),
		),
		time.Now(),
		period,
		finished,
	)
}

// calculates and sends MinMaxLast-s from TradeDatePrice-s from a start date per each resolution
func calculatePriceStats(chDatePrice chan TradeDatePrice, startDate time.Time, resolution time.Duration, finished chan struct{}) chan AggregatedTradeInfo {
	lastSentDate := startDate
	var curMinMaxLast AggregatedTradeInfo
	curMinMaxLast.SetDefault()

	out := make(chan AggregatedTradeInfo)
	go func() {
		defer func() { finished <- struct{}{} }()
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

func unmarshalTradeDatePrice(inp chan string) chan TradeDatePrice {
	out := make(chan TradeDatePrice)
	go func() {
		defer close(out)
		for msg := range inp {
			var msgStruct TradeDatePrice
			err := json.Unmarshal([]byte(msg), &msgStruct)
			if err != nil {
				log.Println("Failed to unmarshal to TradeDatePrice:", err)
				continue
			}
			out <- msgStruct
		}
	}()
	return out
}

func subscribeRedis(subCh string, done chan struct{}) chan string {
	var rdb = redis.NewClient(&redis.Options{
		Addr: RedisAddress,
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
					log.Println("Stopping subscription:", subCh)
					return
				}
			}
		}
	}()
	return out
}
