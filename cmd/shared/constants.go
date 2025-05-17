package shared

import "time"

const (
	// cacher
	RedisAddress = "localhost:6379"
	RedisChannel = "binance:trade:btcusdt"
	// fetcher
	TimeBeforeReconnect = 5 * time.Second // 300 connections per 5 minutes is the limit. this should be fine
	TimeoutBeforeReturn = 5 * time.Second // arbitrary. gets done <1ms, I don't think it's over network
)
