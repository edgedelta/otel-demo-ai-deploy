package main

import "testing"

// PlaceOrder increments ordersByCurrency; a nil map would panic.
func TestOrdersByCurrencyInitialized(t *testing.T) {
	ordersByCurrencyMu.Lock()
	defer ordersByCurrencyMu.Unlock()
	ordersByCurrency["USD"]++
	if ordersByCurrency["USD"] < 1 {
		t.Fatal("expected USD count to be incremented")
	}
}
