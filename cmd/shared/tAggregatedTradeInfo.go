package shared

import (
	"math"
	"time"
)

type AggregatedTradeInfo struct {
	TimeFirst time.Time
	TimeLast  time.Time
	Min       float64
	Max       float64
	First     float64
	Last      float64
}

func (s *AggregatedTradeInfo) GetDefault() AggregatedTradeInfo {
	return AggregatedTradeInfo{
		TimeFirst: time.Time{},
		TimeLast:  time.Time{},
		Min:       math.Inf(1),
		Max:       math.Inf(-1),
		First:     math.NaN(),
		Last:      math.NaN()}
}

func (s *AggregatedTradeInfo) SetDefault() {
	*s = s.GetDefault()
}

func (s *AggregatedTradeInfo) IsDefault() bool {
	def := s.GetDefault()
	return s.TimeFirst == def.TimeFirst &&
		s.TimeLast == def.TimeLast &&
		s.Min == def.Min &&
		s.Max == def.Max &&
		math.IsNaN(s.First) && math.IsNaN(def.First) &&
		math.IsNaN(s.Last) && math.IsNaN(def.Last)
}

func (s *AggregatedTradeInfo) Update(d time.Time, v float64) {
	if s.IsDefault() {
		s.TimeFirst = d
		s.First = v
	}
	s.TimeLast = d
	if s.Min > v {
		s.Min = v
	}
	if s.Max < v {
		s.Max = v
	}
	s.Last = v
}
