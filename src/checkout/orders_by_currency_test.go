package main

import "testing"

// Regression test: ordersByCurrency must be initialised, otherwise the
// increment done in PlaceOrder panics with "assignment to entry in nil map".
func TestOrdersByCurrencyIncrement(t *testing.T) {
	ordersByCurrencyMu.Lock()
	defer ordersByCurrencyMu.Unlock()
	ordersByCurrency["USD"]++
	if got := ordersByCurrency["USD"]; got < 1 {
		t.Fatalf("expected USD count >= 1, got %d", got)
	}
}
