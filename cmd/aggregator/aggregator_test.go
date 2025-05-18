package main

import (
	"testing"
)

func TestDummy(t *testing.T) {
	got := 1
	want := 1
	if got != want {
		t.Errorf("Executed. got %v; want %v", got, want)
	}

}
