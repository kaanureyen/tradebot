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
	ch          chan AggregatedTradeInfo // in
	Period      time.Duration            // in&out (param)
	RedisChName string                   // in&out (param)
	Value       AggregatedTradeInfo      // out
}

type SlicePeriodicStats []PeriodicStats

func (s *SlicePeriodicStats) Add(subCh string, period time.Duration, shutdownOrchestrator *ShutdownOrchestrator) {
	*s = append(*s,
		PeriodicStats{
			ch:          periodicPriceStats(subCh, period, shutdownOrchestrator),
			Period:      period,
			RedisChName: subCh,
			Value:       AggregatedTradeInfo{},
		},
	)
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

func periodicPriceStats(subCh string, period time.Duration, shutdownOrchestrator *ShutdownOrchestrator) chan AggregatedTradeInfo {
	stop, finished := shutdownOrchestrator.Get()
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

// calculates and sends AggregateTradeInfo-s from TradeDatePrice-s from a start date per each resolution
func calculatePriceStats(chDatePrice chan TradeDatePrice, startDate time.Time, resolution time.Duration, finished chan struct{}) chan AggregatedTradeInfo {
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
				log.Println("Error while parsing price as float: ", err)
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

// accepts redis channel name to connect. returns redis message receive channel
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
