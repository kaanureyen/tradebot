package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared/constants"
	"github.com/redis/go-redis/v9"
)

type MinMaxLast struct {
	Min  float64
	Max  float64
	Last float64
}

func (s *MinMaxLast) GetDefault() MinMaxLast {
	return MinMaxLast{Min: math.Inf(1), Max: math.Inf(-1), Last: math.NaN()}
}

func (s *MinMaxLast) SetDefault() {
	*s = s.GetDefault()
}

func (s *MinMaxLast) IsDefault() bool {
	def := s.GetDefault()
	return s.Min == def.Min &&
		s.Max == def.Max &&
		math.IsNaN(s.Last) && math.IsNaN(def.Last)
}

func (s *MinMaxLast) Update(v float64) {
	if s.Min > v {
		s.Min = v
	}
	if s.Max < v {
		s.Max = v
	}
	s.Last = v
}

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
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	// Create a channel to signal when to disconnect from Binance
	subDone := make(chan struct{})
	go func() {
		defer close(subDone)
		sig := <-shutdownSignals
		log.Println("Received int/term signal, will quit:", sig)
		subDone <- struct{}{} // tell Binance to stop
	}()

	chMinMaxLast :=
		calculateMinMaxLast(
			unmarshalTradeDatePrice(
				subscribe(
					constants.RedisChannel,
					subDone,
				),
			),
			time.Now(),
			10*time.Second,
		)

	for v := range chMinMaxLast {
		log.Println(v)
	}

	log.Println("Exiting...")
}

// calculates and sends MinMaxLast-s from TradeDatePrice-s from a start date per each resolution
func calculateMinMaxLast(chDatePrice chan constants.TradeDatePrice, startDate time.Time, resolution time.Duration) chan MinMaxLast {
	lastSentDate := startDate
	var curMinMaxLast MinMaxLast
	curMinMaxLast.SetDefault()

	out := make(chan MinMaxLast)
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
				curMinMaxLast.Update(p)
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
			for msg := range ch {
				out <- msg.Payload

				select {
				case <-done:
					log.Println("Stopping subscription:", subCh)
					return
				default:
					// continue
				}
			}
		}
	}()
	return out
}
