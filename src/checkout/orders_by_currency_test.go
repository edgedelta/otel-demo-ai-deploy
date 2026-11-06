package main

import "testing"

func TestOrdersByCurrencyIncrementDoesNotPanic(t *testing.T) {
	ordersByCurrencyMu.Lock()
	ordersByCurrency["USD"]++
	got := ordersByCurrency["USD"]
	ordersByCurrencyMu.Unlock()
	if got < 1 {
		t.Fatalf("expected USD count >= 1, got %d", got)
	}
}
