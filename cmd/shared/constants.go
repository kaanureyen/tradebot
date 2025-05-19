package shared

import "time"

const (
	// common
	HealthEndpointFirstPort = 8080
	HealthEndpointLastPort  = 8100
	// aggregator
	RedisChannel = "binance:trade:btcusdt"
	SmaLongTerm  = 200
	SmaShortTerm = 50
	// fetcher
	TimeBeforeReconnect = 5 * time.Second // 300 connections per 5 minutes is the limit. this should be fine
	TimeoutBeforeReturn = 5 * time.Second // arbitrary. gets done <1ms, I don't think it's over network
)

// not a constant but only known in runtime.
var (
	RedisAddress = func() string { // detect whether running under docker in runtime
		if IsRunningInDocker() {
			return "redis:6379"
		} else {
			return "localhost:6379"
		}
	}()
	MongoUri = func() string { // detect whether running under docker in runtime
		if IsRunningInDocker() {
			return "mongodb://mongodb:27017"
		} else {
			return "mongodb://localhost:27017"
		}
	}()
)
