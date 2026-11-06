package main

import "testing"

func TestRecordOrderCurrencyDoesNotPanic(t *testing.T) {
	ordersByCurrencyMu.Lock()
	before := ordersByCurrency["USD"]
	ordersByCurrencyMu.Unlock()

	recordOrderCurrency("USD")
	recordOrderCurrency("USD")

	ordersByCurrencyMu.Lock()
	got := ordersByCurrency["USD"]
	ordersByCurrencyMu.Unlock()
	if got != before+2 {
		t.Fatalf("expected %d, got %d", before+2, got)
	}
}
