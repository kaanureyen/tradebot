package main

import (
	"errors"
	"log"
	"math"
	"time"
)

type SmaBuffer struct {
	prices    []float64
	dates     []time.Time
	size      int
	pos       int
	dataCount int
}

func (s *SmaBuffer) Init(size int) {
	s.prices = make([]float64, size)
	s.dates = make([]time.Time, size)
	s.size = size
	s.pos = 0
	s.dataCount = 0
}

// gets relative index. +1 for next, -1 for previous.
func (s *SmaBuffer) relInd(rel int) int {
	return ((s.pos+rel)%s.size + s.size) % s.size
}

// gets absolute index. 0 for first, +1 for second, -1 for last.
func (s *SmaBuffer) absInd(abs int) int {
	return ((abs)%s.size + s.size) % s.size
}

func (s *SmaBuffer) Add(price float64, date time.Time) {
	s.pos = s.relInd(1)
	s.prices[s.pos] = price
	s.dates[s.pos] = date
	if s.dataCount < s.size {
		s.dataCount++
	}
}

// checks the need for linear interpolation. if it is needed, does it prior adding new data.
func (s *SmaBuffer) AddWithLinInterpFill(price float64, date time.Time, period time.Duration) {
	var numPeriodsSinceLast int // how many periods elapsed since the last data
	if s.dataCount > 0 {
		oldDate := s.dates[s.pos]
		oldPrice := s.prices[s.pos]

		deltaDate := date.Sub(oldDate)
		deltaPrice := price - oldPrice

		numPeriodsSinceLast = int(math.Round(float64(deltaDate) / float64(period)))
		if numPeriodsSinceLast > 1 { // more than 1 period elapsed: lininterp
			log.Printf("[Info] The last added SMA data point (price: %v, date: %v) is approximately %v periods after than the last data point. Delta time: %v, Period: %v. Missing data will be linearly interpolated.\n", price, date, numPeriodsSinceLast, deltaDate, period)
			for i := 1; i < numPeriodsSinceLast; i++ {
				interpolatedPrice := oldPrice + (float64(i) * deltaPrice / float64(numPeriodsSinceLast))
				interpolatedDate := oldDate.Add(time.Duration(i) * deltaDate / time.Duration(numPeriodsSinceLast))
				s.Add(interpolatedPrice, interpolatedDate)
			}
		}
	}
	s.Add(price, date)
}

func (s *SmaBuffer) IsSmaReady(n int) bool {
	return n <= s.dataCount
}

func (s *SmaBuffer) CalculateSma(n int) (float64, error) {
	if !s.IsSmaReady(n) {
		return math.NaN(), errors.New("insufficient data for SMA calculation")
	}
	sum := 0.0
	for i, k := s.pos, 0; k < n; i, k = s.absInd(i-1), k+1 {
		sum += s.prices[i]
	}
	return sum / float64(n), nil
}
