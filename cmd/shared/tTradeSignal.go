package shared

import "time"

type TradeSignal struct {
	TimeStamp time.Time `bson:"timestamp"`
	Signal    string    `bson:"signal"`
	Price     float64   `bson:"price"`
	Sma50     float64   `bson:"sma50"`
	Sma200    float64   `bson:"sma200"`
}
