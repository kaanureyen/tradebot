//go:build integration
// +build integration

package integration

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Do setup here

	// Run all TestXxx Functions
	code := m.Run()

	// Do teardown here

	// Exit with the success code
	os.Exit(code)
}

func TestDummy(t *testing.T) {
	got := 1
	want := 1
	if got != want {
		t.Errorf("Executed. got %v; want %v", got, want)
	}

}
