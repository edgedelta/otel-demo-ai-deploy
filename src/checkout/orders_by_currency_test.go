package main

import "testing"

// Without initializing ordersByCurrency this panics with
// "assignment to entry in nil map", as PlaceOrder does.
func TestOrdersByCurrencyIsInitialized(t *testing.T) {
	ordersByCurrencyMu.Lock()
	defer ordersByCurrencyMu.Unlock()
	ordersByCurrency["USD"]++
	if ordersByCurrency["USD"] < 1 {
		t.Fatalf("expected USD count >= 1")
	}
}
