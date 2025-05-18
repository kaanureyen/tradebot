package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared"
	"github.com/redis/go-redis/v9"

	binance_connector "github.com/binance/binance-connector-go"
)

var rdb = redis.NewClient(&redis.Options{
	Addr: shared.RedisAddress,
})
var ctx = context.Background()

func main() {
	shutdownOrchestrator := shared.InitCommon("fetcher") // set logger name, start http health endpoint, initialize & start shutdownOrchestrator
	defer func() {
		<-shutdownOrchestrator.Done // blocks until every shutdownOrchestrator.Get()'s recv is sent an empty struct, after a interrupt/terminate signal.
		log.Println("Exiting...")
	}()

	// fetch data from binance & publish on redis
	fetchAndPublish("BTCUSDT", shutdownOrchestrator, tradeEvent, errorEvent)
}

func tradeEvent(event *binance_connector.WsTradeEvent) {
	data, err := json.Marshal(shared.TradeDatePrice{TradeDate: event.TradeTime, Price: event.Price})
	if err != nil {
		log.Fatalln("Error marshalling data.\nerr:", err, "\ndata:", data)
		return
	}

	err = rdb.Publish(ctx, shared.RedisChannel, data).Err()
	if err != nil {
		log.Println("Redis Publish error:", err)
		return
	}
}

func errorEvent(err error) {
	log.Println("Error in Websocket stream:", err)
}

func fetchAndPublish(exchange string, shutdownOrchestrator *shared.ShutdownOrchestrator, handleTradeEvent func(*binance_connector.WsTradeEvent), handleErrorEvent func(error)) {
	stop, done := shutdownOrchestrator.Get() // get stop and done signals
	defer func() { done <- struct{}{} }()    // tell orchestrator this is done

	for { // connection will drop. reconnect when happens
		log.Println("Connecting to Binance")
		// connect to Binance Trade Websocket stream
		websocketStreamClient := binance_connector.NewWebsocketStreamClient(false)
		doneCh, stopCh, err := websocketStreamClient.WsTradeServe(exchange, handleTradeEvent, handleErrorEvent)
		if err != nil {
			log.Println("Error while opening Websocket stream:", err)
			log.Println("Retrying in:", shared.TimeBeforeReconnect)
			time.Sleep(shared.TimeBeforeReconnect) // wait before retrying
			continue                               // retry
		}
		log.Println("Connected to Binance")

		// Wait for the WS stream to close OR quit signal
		select {
		case <-doneCh: // Binance is done, but we are not
			log.Println("Binance connection closed, reconnecting in:", shared.TimeBeforeReconnect)
			time.Sleep(shared.TimeBeforeReconnect)
			continue // reconnect

		case <-stop: // stop command from shutdown orchestrator
			log.Println("Telling Binance to quit.")
			stopCh <- struct{}{}
			log.Println("Waiting for Binance to close connection.")

			// Wait for Binance to close connection OR timeout
			select {
			case <-doneCh:
				log.Println("Binance connection is closed normally.")

			case <-time.After(shared.TimeoutBeforeReturn):
				log.Println("Timeout (", shared.TimeoutBeforeReturn, ") waiting for Binance to close connection")
			}
			return
		}
	}
}
