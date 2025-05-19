package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// prometheus metrics
var aggregateInfoAge = prometheus.NewSummary(
	prometheus.SummaryOpts{
		Name:       "aggregate_info_age_milliseconds",
		Help:       "Difference of local time on aggregate creation and trade time in milliseconds",
		Objectives: map[float64]float64{0.5: 0.05, 0.95: 0.01, 0.99: 0.001},
	},
)
var aggregatePrice = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "aggregate_info_price",
		Help: "BTC-USDT Price",
	},
)
var aggregateSma50 = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "aggregate_info_sma50",
		Help: "BTC-USDT Price SMA50",
	},
)
var aggregateSma200 = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "aggregate_info_sma200",
		Help: "BTC-USDT Price SMA200",
	},
)
var aggregateSell = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "aggregate_info_sell_count",
		Help: "Sell Count",
	},
)
var aggregateBuy = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "aggregate_info_buy_count",
		Help: "Buy Count",
	},
)

type SmaStruct struct {
	TimeStamp time.Time `bson:"timestamp"`
	Sma50     float64   `bson:"sma50"`
	Sma200    float64   `bson:"sma200"`
}

type TradeSignal struct {
	TimeStamp time.Time `bson:"timestamp"`
	Signal    string    `bson:"signal"`
	Price     float64   `bson:"price"`
	Sma50     float64   `bson:"sma50"`
	Sma200    float64   `bson:"sma200"`
}

func main() {
	shutdownOrchestrator := shared.InitCommon("aggregator") // set logger name, start http health endpoint, initialize & start shutdownOrchestrator
	defer func() {
		<-shutdownOrchestrator.Done // blocks until every shutdownOrchestrator.Get()'s recv is sent an empty struct, after a interrupt/terminate signal.
		log.Println("[Info] Exiting...")
	}()

	// register the prometheus metrics
	prometheus.MustRegister(aggregateInfoAge)
	prometheus.MustRegister(aggregatePrice)
	prometheus.MustRegister(aggregateSma50)
	prometheus.MustRegister(aggregateSma200)
	prometheus.MustRegister(aggregateSell)
	prometheus.MustRegister(aggregateBuy)
	// start prometheus metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal("[Fatal][Error] Prometheus metrics endpoint could not be opened. Error: ", http.ListenAndServe(":2113", nil))
	}()

	// connect to MongoDB
	client, ctx := MongoConnect()
	defer client.Disconnect(ctx)

	collAggr := MongoAggregateCollection(client, ctx)
	collSma := MongoSmaCollection(client, ctx)
	collTrade := MongoTradeCollection(client, ctx)

	// price bucketing period
	period := time.Second

	// initialize sma buffer
	smaBuffer := SmaBuffer{}
	smaBuffer.Init(shared.SmaLongTerm)

	// load into sma buffer from DB
	log.Println("[Info] Loading the last price data from the DB")
	LoadLastNIntoSmaBuffer(collAggr, shared.SmaLongTerm, &smaBuffer, period)

	// start read from Redis
	log.Println("[Info] Start reading price data from Redis")
	aggCh := PeriodicPriceStats(shared.RedisChannel, period, shutdownOrchestrator)

	lastDiff := 0.0
	for v := range aggCh {
		// Store to MongoDB time series
		_, err := collAggr.InsertOne(ctx, v)
		if err != nil {
			log.Printf("[Error] Failed to insert to MongoDB: %v\n", err)
		}

		smaBuffer.AddWithLinInterpFill(v.LastPrice, v.LastTime, period)

		if smaBuffer.IsSmaReady(shared.SmaLongTerm) {
			smaShortTerm, _ := smaBuffer.CalculateSma(shared.SmaShortTerm)
			smaLongTerm, _ := smaBuffer.CalculateSma(shared.SmaLongTerm)

			tradeSignal := TradeSignal{
				TimeStamp: time.Now(),
				Signal:    "",
				Price:     v.LastPrice,
				Sma50:     smaShortTerm,
				Sma200:    smaLongTerm,
			}

			diff := smaShortTerm - smaLongTerm
			if diff > 0 && lastDiff <= 0 {
				tradeSignal.Signal = "BUY"
				aggregateBuy.Inc()
			}
			if diff < 0 && lastDiff >= 0 {
				tradeSignal.Signal = "SELL"
				aggregateSell.Inc()
			}
			if tradeSignal.Signal != "" {
				// Store to MongoDB time series
				_, err := collTrade.InsertOne(ctx, tradeSignal)
				if err != nil {
					log.Printf("[Error] Failed to insert to MongoDB: %v\n", err)
				}
			}
			lastDiff = diff
			aggregateInfoAge.Observe(float64(time.Since(v.LastTime).Milliseconds()))
			aggregatePrice.Set(v.LastPrice)
			aggregateSma200.Set(smaLongTerm)
			aggregateSma50.Set(smaShortTerm)

			// Store to MongoDB time series
			_, err := collSma.InsertOne(ctx, SmaStruct{
				TimeStamp: v.LastTime,
				Sma50:     smaShortTerm,
				Sma200:    smaLongTerm,
			})
			if err != nil {
				log.Printf("[Error] Failed to insert to MongoDB: %v\n", err)
			}
		}
	}
}

// Example function to get last N items
func LoadLastNIntoSmaBuffer(collection *mongo.Collection, n int, smaBuffer *SmaBuffer, period time.Duration) {
	ctx := context.Background()
	opts := options.Find().SetSort(bson.D{{Key: "lasttimestamp", Value: -1}}).SetLimit(int64(n))
	cursor, err := collection.Find(ctx, bson.D{}, opts)
	if err != nil {
		log.Printf("[Error] Cannot find from MongoDB and will continue without loading from DB: %v\n", err)
		return
	}
	defer cursor.Close(ctx)

	var results []AggregatedTradeInfo
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("[Error] Cannot load from cursor and will continue without loading from DB: %v\n", err)
		return
	}

	for i := len(results) - 1; i >= 0; i-- {
		v := results[i]
		smaBuffer.AddWithLinInterpFill(v.LastPrice, v.LastTime, period)
		log.Printf("[Debug] Loaded from DB: Price %v Time %v\n", v.LastPrice, v.LastTime)
	}
}

func MongoConnect() (*mongo.Client, context.Context) {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(shared.MongoUri))
	if err != nil {
		log.Fatal(err)
	}
	return client, ctx
}

func MongoAggregateCollection(client *mongo.Client, ctx context.Context) *mongo.Collection {
	// try to create timeseries collection.
	opts := options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().
			SetTimeField("lasttimestamp").
			SetMetaField("btcusdt:1s"),
	)

	err := client.Database("tradebot").CreateCollection(ctx, "price_stats", opts)
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database("tradebot").Collection("price_stats")

	// set indexing to descending
	_, err = collection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{
					Key:   "lasttimestamp",
					Value: -1},
			}, // descending index
			Options: nil,
		},
	)
	if err != nil {
		log.Fatalf("[Error] Failed to create index: %v\n", err)
	}
	return collection
}

func MongoSmaCollection(client *mongo.Client, ctx context.Context) *mongo.Collection {
	// try to create timeseries collection.
	opts := options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().
			SetTimeField("timestamp").
			SetMetaField("btcusdt:1s:sma"),
	)

	err := client.Database("tradebot").CreateCollection(ctx, "price_stats_sma", opts)
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database("tradebot").Collection("price_stats_sma")

	// set indexing to descending
	_, err = collection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{
					Key:   "timestamp",
					Value: -1},
			}, // descending index
			Options: nil,
		},
	)
	if err != nil {
		log.Fatalf("[Error] Failed to create index: %v\n", err)
	}
	return collection
}

func MongoTradeCollection(client *mongo.Client, ctx context.Context) *mongo.Collection {
	// try to create timeseries collection.
	opts := options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().
			SetTimeField("timestamp").
			SetMetaField("btcusdt:1s:sma:trade"),
	)

	err := client.Database("tradebot").CreateCollection(ctx, "price_stats_sma_trade", opts)
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database("tradebot").Collection("price_stats_sma_trade")

	// set indexing to descending
	_, err = collection.Indexes().CreateOne(
		ctx,
		mongo.IndexModel{
			Keys: bson.D{
				{
					Key:   "timestamp",
					Value: -1},
			}, // descending index
			Options: nil,
		},
	)
	if err != nil {
		log.Fatalf("[Error] Failed to create index: %v\n", err)
	}
	return collection
}
