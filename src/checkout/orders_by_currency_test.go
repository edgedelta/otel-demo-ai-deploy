package main

import "testing"

// Incrementing the per-currency counter, as PlaceOrder does, must not panic
// with "assignment to entry in nil map".
func TestOrdersByCurrencyInitialised(t *testing.T) {
	ordersByCurrencyMu.Lock()
	defer ordersByCurrencyMu.Unlock()
	ordersByCurrency["USD"]++
	if ordersByCurrency["USD"] < 1 {
		t.Fatalf("expected USD count >= 1, got %d", ordersByCurrency["USD"])
	}
}
