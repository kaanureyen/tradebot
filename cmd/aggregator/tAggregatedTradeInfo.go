package main

import (
	"math"
	"time"
)

type AggregatedTradeInfo struct {
	FirstTime  time.Time `bson:"firsttimestamp"`
	LastTime   time.Time `bson:"lasttimestamp"`
	MinPrice   float64   `bson:"min_price"`
	MaxPrice   float64   `bson:"max_price"`
	FirstPrice float64   `bson:"first_price"`
	LastPrice  float64   `bson:"last_price"`
}

func (s *AggregatedTradeInfo) getDefault() AggregatedTradeInfo {
	return AggregatedTradeInfo{
		FirstTime:  time.Time{},
		LastTime:   time.Time{},
		MinPrice:   math.Inf(1),
		MaxPrice:   math.Inf(-1),
		FirstPrice: math.NaN(),
		LastPrice:  math.NaN()}
}

func (s *AggregatedTradeInfo) SetDefault() {
	*s = s.getDefault()
}

func (s *AggregatedTradeInfo) IsDefault() bool {
	def := s.getDefault()
	return s.FirstTime == def.FirstTime &&
		s.LastTime == def.LastTime &&
		s.MinPrice == def.MinPrice &&
		s.MaxPrice == def.MaxPrice &&
		math.IsNaN(s.FirstPrice) && math.IsNaN(def.FirstPrice) &&
		math.IsNaN(s.LastPrice) && math.IsNaN(def.LastPrice)
}

func (s *AggregatedTradeInfo) Update(d time.Time, v float64) {
	if s.IsDefault() {
		s.FirstTime = d
		s.FirstPrice = v
	}
	s.LastTime = d
	if s.MinPrice > v {
		s.MinPrice = v
	}
	if s.MaxPrice < v {
		s.MaxPrice = v
	}
	s.LastPrice = v
}
