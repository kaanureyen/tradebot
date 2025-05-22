package main

import (
	"context"
	"log"

	"github.com/kaanureyen/tradebot/cmd/shared"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	shutdownOrchestrator := shared.InitCommon("simulator") // set logger name, start http health endpoint, initialize & start shutdownOrchestrator
	defer func() {
		<-shutdownOrchestrator.Done // blocks until every shutdownOrchestrator.Get()'s recv is sent an empty struct, after a interrupt/terminate signal.
		log.Println("[Info] Exiting...")
	}()

	// connect to MongoDB
	client, ctx := shared.MongoConnect()
	defer client.Disconnect(ctx)

	collTrade := shared.MongoTradeCollection(client, ctx)
	w := Wallet{
		BTC:  0,
		USDT: 1000,
	}
	SimulateLastN(collTrade, 999999, w)
}

type Wallet struct {
	BTC  float64
	USDT float64
}

func (w *Wallet) BuyAll(price float64) {
	btcToBuy := w.USDT / price
	w.BTC += btcToBuy
	w.USDT = 0
}

func (w *Wallet) SellAll(price float64) {
	usdtToBuy := w.BTC * price
	w.USDT += usdtToBuy
	w.BTC = 0
}

// Example function to get last N items
func SimulateLastN(collection *mongo.Collection, n int, startWallet Wallet) {
	ctx := context.Background()
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).SetLimit(int64(n))
	cursor, err := collection.Find(ctx, bson.D{}, opts)
	if err != nil {
		log.Printf("[Error] Cannot find from MongoDB: %v\n", err)
		return
	}
	defer cursor.Close(ctx)

	var results []shared.TradeSignal
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("[Error] Cannot load from cursor: %v\n", err)
		return
	}

	for i := len(results) - 1; i >= 0; i-- {
		v := results[i]

		log.Printf("Time: %v Price: %v Action: %v\n", v.TimeStamp, v.Price, v.Signal)
		log.Printf("Old Wallet:%v\n", startWallet)

		if v.Signal == "BUY" {
			startWallet.BuyAll(v.Price)
		} else if v.Signal == "SELL" {
			startWallet.SellAll(v.Price)
		}

		log.Printf("New Wallet:%v\n", startWallet)

	}
}
