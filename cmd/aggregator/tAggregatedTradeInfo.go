package main

import (
	"math"
	"time"
)

type AggregatedTradeInfo struct {
	firstTime  time.Time `bson:"firsttimestamp"`
	lastTime   time.Time `bson:"lasttimestamp"`
	minPrice   float64   `bson:"min_price"`
	maxPrice   float64   `bson:"max_price"`
	firstPrice float64   `bson:"first_price"`
	lastPrice  float64   `bson:"last_price"`
}

func (s *AggregatedTradeInfo) getDefault() AggregatedTradeInfo {
	return AggregatedTradeInfo{
		firstTime:  time.Time{},
		lastTime:   time.Time{},
		minPrice:   math.Inf(1),
		maxPrice:   math.Inf(-1),
		firstPrice: math.NaN(),
		lastPrice:  math.NaN()}
}

func (s *AggregatedTradeInfo) SetDefault() {
	*s = s.getDefault()
}

func (s *AggregatedTradeInfo) IsDefault() bool {
	def := s.getDefault()
	return s.firstTime == def.firstTime &&
		s.lastTime == def.lastTime &&
		s.minPrice == def.minPrice &&
		s.maxPrice == def.maxPrice &&
		math.IsNaN(s.firstPrice) && math.IsNaN(def.firstPrice) &&
		math.IsNaN(s.lastPrice) && math.IsNaN(def.lastPrice)
}

func (s *AggregatedTradeInfo) Update(d time.Time, v float64) {
	if s.IsDefault() {
		s.firstTime = d
		s.firstPrice = v
	}
	s.lastTime = d
	if s.minPrice > v {
		s.minPrice = v
	}
	if s.maxPrice < v {
		s.maxPrice = v
	}
	s.lastPrice = v
}
