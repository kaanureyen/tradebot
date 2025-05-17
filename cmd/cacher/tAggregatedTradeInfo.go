package main

import (
	"math"
	"time"
)

type AggregatedTradeInfo struct {
	timeFirst time.Time
	timeLast  time.Time
	min       float64
	max       float64
	first     float64
	last      float64
}

func (s *AggregatedTradeInfo) getDefault() AggregatedTradeInfo {
	return AggregatedTradeInfo{
		timeFirst: time.Time{},
		timeLast:  time.Time{},
		min:       math.Inf(1),
		max:       math.Inf(-1),
		first:     math.NaN(),
		last:      math.NaN()}
}

func (s *AggregatedTradeInfo) SetDefault() {
	*s = s.getDefault()
}

func (s *AggregatedTradeInfo) IsDefault() bool {
	def := s.getDefault()
	return s.timeFirst == def.timeFirst &&
		s.timeLast == def.timeLast &&
		s.min == def.min &&
		s.max == def.max &&
		math.IsNaN(s.first) && math.IsNaN(def.first) &&
		math.IsNaN(s.last) && math.IsNaN(def.last)
}

func (s *AggregatedTradeInfo) Update(d time.Time, v float64) {
	if s.IsDefault() {
		s.timeFirst = d
		s.first = v
	}
	s.timeLast = d
	if s.min > v {
		s.min = v
	}
	if s.max < v {
		s.max = v
	}
	s.last = v
}
