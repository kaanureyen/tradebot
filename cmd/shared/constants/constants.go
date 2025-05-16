package constants

import "math"

const (
	RedisAddress = "localhost:6379"
	RedisChannel = "binance:trade:btcusdt"
)

type TradeDatePrice struct {
	TradeDate int64
	Price     string
}

type MinMaxLast struct {
	Min  float64
	Max  float64
	Last float64
}

func (s *MinMaxLast) GetDefault() MinMaxLast {
	return MinMaxLast{Min: math.Inf(1), Max: math.Inf(-1), Last: math.NaN()}
}

func (s *MinMaxLast) SetDefault() {
	*s = s.GetDefault()
}

func (s *MinMaxLast) IsDefault() bool {
	def := s.GetDefault()
	return s.Min == def.Min &&
		s.Max == def.Max &&
		math.IsNaN(s.Last) && math.IsNaN(def.Last)
}

func (s *MinMaxLast) Update(v float64) {
	if s.Min > v {
		s.Min = v
	}
	if s.Max < v {
		s.Max = v
	}
	s.Last = v
}
