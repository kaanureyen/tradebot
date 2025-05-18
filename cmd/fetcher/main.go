package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared"
	"github.com/redis/go-redis/v9"

	binance_connector "github.com/binance/binance-connector-go"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// global redis & context to publish data
var rdb = redis.NewClient(&redis.Options{
	Addr: shared.RedisAddress,
})
var ctx = context.Background()

// prometheus counter for metrics
var tradesReceived = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "trades_received_total",
		Help: "Total number of trades received from Binance.",
	},
)
var tradesPublished = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "trades_published_total",
		Help: "Total number of trades published to Redis.",
	},
)

var tradeEventDelay = prometheus.NewSummary(
	prometheus.SummaryOpts{
		Name: "trade_event_delay_milliseconds",
		Help: "Time taken to process and publish a trade event",
	},
)

var tradeInfoAge = prometheus.NewSummary(
	prometheus.SummaryOpts{
		Name: "trade_info_age_milliseconds",
		Help: "Difference of local time and trade time in milliseconds",
	},
)

func main() {
	shutdownOrchestrator := shared.InitCommon("fetcher") // set logger name, start http health endpoint, initialize & start shutdownOrchestrator
	defer func() {
		<-shutdownOrchestrator.Done // blocks until every shutdownOrchestrator.Get()'s recv is sent an empty struct, after a interrupt/terminate signal.
		log.Println("[Info] Exiting...")
	}()

	// register the prometheus metric
	prometheus.MustRegister(tradesReceived)
	prometheus.MustRegister(tradesPublished)
	prometheus.MustRegister(tradeEventDelay)
	prometheus.MustRegister(tradeInfoAge)
	// start prometheus metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal("[Fatal][Error] Prometheus metrics endpoint could not be opened. Error: ", http.ListenAndServe(":2112", nil))
	}()

	// fetch data from binance & publish on redis
	fetchAndPublish("BTCUSDT", shutdownOrchestrator, tradeEvent, errorEvent)
}

func tradeEvent(event *binance_connector.WsTradeEvent) {
	// prepare & update stats
	start := time.Now()
	tradesReceived.Inc()
	tradeInfoAge.Observe(float64(time.Now().UnixMilli() - event.TradeTime))

	// marshal into json
	data, err := json.Marshal(shared.TradeDatePrice{TradeDate: event.TradeTime, Price: event.Price})
	if err != nil {
		log.Printf("[Warning] Failed marshaling data. Skipping data.\nErr: %v\nData: %v", err, data)
		return
	}

	// publish into redis
	err = rdb.Publish(ctx, shared.RedisChannel, data).Err()
	if err != nil {
		log.Println("[Warning] Redis Publish error:", err)
		return
	}

	// update stats
	tradesPublished.Inc()
	tradeEventDelay.Observe(float64(time.Since(start).Nanoseconds()))
}

func errorEvent(err error) {
	log.Println("[Warning] Error in Websocket stream:", err)
}

func fetchAndPublish(exchange string, shutdownOrchestrator *shared.ShutdownOrchestrator, handleTradeEvent func(*binance_connector.WsTradeEvent), handleErrorEvent func(error)) {
	stop, done := shutdownOrchestrator.Get() // get stop and done signals
	defer func() { done <- struct{}{} }()    // tell orchestrator this is done

	for { // connection will drop. reconnect when happens
		log.Println("[Info] Connecting to Binance")
		// connect to Binance Trade Websocket stream
		websocketStreamClient := binance_connector.NewWebsocketStreamClient(false)
		doneCh, stopCh, err := websocketStreamClient.WsTradeServe(exchange, handleTradeEvent, handleErrorEvent)
		if err != nil {
			log.Println("[Warning] Error while opening Websocket stream:", err)
			log.Println("[Info] Retrying in:", shared.TimeBeforeReconnect)
			time.Sleep(shared.TimeBeforeReconnect) // wait before retrying
			continue                               // retry
		}
		log.Println("[Info] Connected to Binance")

		// Wait for the WS stream to close OR quit signal
		select {
		case <-doneCh: // Binance is done, but we are not
			log.Println("[Warning] Binance connection closed, reconnecting in:", shared.TimeBeforeReconnect)
			time.Sleep(shared.TimeBeforeReconnect)
			continue // reconnect

		case <-stop: // stop command from shutdown orchestrator
			log.Println("[Info] Telling Binance to quit.")
			stopCh <- struct{}{}
			log.Println("[Info] Waiting for Binance to close connection.")

			// Wait for Binance to close connection OR timeout
			select {
			case <-doneCh:
				log.Println("[Info] Binance connection is closed normally.")

			case <-time.After(shared.TimeoutBeforeReturn):
				log.Printf("[Warning] Timeout (%v) waiting for Binance to close connection", shared.TimeoutBeforeReturn)
			}
			return
		}
	}
}
