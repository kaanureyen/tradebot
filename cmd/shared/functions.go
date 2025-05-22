package shared

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func IsRunningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// initializes logger, starts health endpoint, inits&returns pointer to a shutdownOrchestrator
func InitCommon(moduleName string) *ShutdownOrchestrator {
	logger("[" + moduleName + "] ") // set logger and print start msg
	healthEndpoint(moduleName)      // start health endpoint and print msg

	// start shutdown orchestrator
	var shutdownOrchestrator ShutdownOrchestrator
	shutdownOrchestrator.Start()
	return &shutdownOrchestrator
}

func logger(s string) {
	// show line number in logs, show microseconds, add prefix
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix(s)
	log.Println("[Info] Started")
}

func healthEndpoint(moduleName string) {
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(moduleName + " is OK"))
		})

		port := os.Getenv("HEALTH_PORT")
		if port != "" { // found HEALTH_PORT. will only try it.
			log.Printf("[Info] Health endpoint (http://localhost:%v/healthz) listening on :%v\n", port, port)
			if err := http.ListenAndServe(":"+port, nil); err != nil {
				log.Fatalf("[Fatal][Error] Error starting health endpoint: %v", err)
			}

		} else { // will try ports in range [HealthEndpointFirstPort, HealthEndpointLastPort] one by one
			for port := HealthEndpointFirstPort; port < HealthEndpointLastPort; port++ {
				log.Printf("[Info] Health endpoint (http://localhost:%v/healthz) listening on :%v\n", port, port)
				if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
					log.Printf("[Warning] Error starting health endpoint: %v", err)
					log.Printf("[Info] Trying next port.")
					continue
				}
				return
			}
			log.Fatalf("[Fatal][Error] Could not find an empty port for health endpoint for %v in range [%v,%v]\n", moduleName, HealthEndpointFirstPort, HealthEndpointLastPort)
		}
	}()
}

func MongoConnect() (*mongo.Client, context.Context) {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(MongoUri))
	if err != nil {
		log.Fatal(err)
	}
	return client, ctx
}

func MongoAggregateCollection(client *mongo.Client, ctx context.Context) *mongo.Collection {
	// try to create timeseries collection.
	opts := options.CreateCollection().SetTimeSeriesOptions(
		options.TimeSeries().
			SetTimeField("lasttimestamp"),
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
			SetTimeField("timestamp"),
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
			SetTimeField("timestamp"),
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
