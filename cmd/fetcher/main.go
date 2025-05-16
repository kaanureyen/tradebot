package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared/constants"
	"github.com/redis/go-redis/v9"

	binance_connector "github.com/binance/binance-connector-go"
)

var rdb = redis.NewClient(&redis.Options{
	Addr: constants.RedisAddress,
})
var ctx = context.Background()

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds) // show line number in logs, show microseconds
	log.SetPrefix("[fetcher] ")
	log.Println("Started")

	// Create a channel to listen for signals from the OS for graceful shutdown
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	// Create a channel to signal when to disconnect from Binance
	disconnectFromBinance := make(chan struct{})
	go func() {
		<-shutdownSignals
		log.Println("Received shutdown signal, will quit.")
		disconnectFromBinance <- struct{}{} // tell Binance to stop
	}()
	stayConnectedToBinance("BTCUSDT", disconnectFromBinance, handleTradeEvent, handleErrorEvent)
	log.Println("Exiting...")
}

func handleTradeEvent(event *binance_connector.WsTradeEvent) {
	data, err := json.Marshal(constants.TradeDatePrice{TradeDate: event.TradeTime, Price: event.Price})
	if err != nil {
		log.Fatalln("Error marshalling data.\nerr:", err, "\ndata:", data)
		return
	}

	err = rdb.Publish(ctx, constants.RedisChannel, data).Err()
	if err != nil {
		log.Println("Redis Publish error:", err)
		return
	}

}

func handleErrorEvent(err error) {
	log.Println("Error in Websocket stream:", err)
}

func stayConnectedToBinance(exchange string, quit chan struct{}, handleTradeEvent func(*binance_connector.WsTradeEvent), handleErrorEvent func(error)) {
	const timeBeforeReconnect = 5 * time.Second // 300 connections per 5 minutes is the limit. this should be fine
	const timeoutBeforeReturn = 5 * time.Second // arbitrary. gets done <1ms, I don't think it's over network
	for {
		log.Println("Connecting to Binance")
		// connect to Binance Trade Websocket streamclosing Binance connection
		websocketStreamClient := binance_connector.NewWebsocketStreamClient(false)
		doneCh, stopCh, err := websocketStreamClient.WsTradeServe(exchange, handleTradeEvent, handleErrorEvent)
		if err != nil {
			log.Println("Error while opening Websocket stream:", err)
			log.Println("Retrying in:", timeBeforeReconnect)
			time.Sleep(timeBeforeReconnect) // wait before retrying
			continue                        // retry
		}
		log.Println("Connected to Binance")

		// Wait for the WS stream to close OR quit signal
		select {
		case <-doneCh: // Binance is done, but we are not
			log.Println("Binance connection closed, reconnecting in:", timeBeforeReconnect)
			time.Sleep(timeBeforeReconnect)
			continue // reconnect

		case <-quit: // we are done
			log.Println("Telling Binance to quit.")
			stopCh <- struct{}{}
			log.Println("Waiting for Binance to close connection.")

			// Wait for Binance to close connection OR timeout
			select {
			case <-doneCh:
				log.Println("Binance connection is closed normally.")

			case <-time.After(timeoutBeforeReturn):
				log.Println("Timeout (", timeoutBeforeReturn, ") waiting for Binance to close connection")
			}
			return
		}
	}
}
