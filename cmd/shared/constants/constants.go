package constants

const (
	RedisAddress = "localhost:6379"
	RedisChannel = "binance:trade:btcusdt"
)

type TradeDatePrice struct {
	TradeDate int64
	Price     string
}
