package main

import "testing"

func TestOrdersByCurrencyIncrementDoesNotPanic(t *testing.T) {
	if ordersByCurrency == nil {
		t.Fatal("ordersByCurrency must be initialised")
	}
	ordersByCurrencyMu.Lock()
	ordersByCurrency["USD"]++
	ordersByCurrency["USD"]++
	ordersByCurrencyMu.Unlock()
	if got := ordersByCurrency["USD"]; got < 2 {
		t.Fatalf("expected at least 2 USD orders, got %d", got)
	}
}
