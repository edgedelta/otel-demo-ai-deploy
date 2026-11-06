package main

import (
	"sync"
	"testing"
)

func TestRecordOrderCurrencyDoesNotPanic(t *testing.T) {
	ordersByCurrencyMu.Lock()
	ordersByCurrency = make(map[string]int)
	ordersByCurrencyMu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recordOrderCurrency("USD")
		}()
	}
	wg.Wait()

	if got := ordersByCurrency["USD"]; got != 50 {
		t.Fatalf("expected 50 USD orders, got %d", got)
	}
}

func TestOrdersByCurrencyInitialized(t *testing.T) {
	// Package-level initial state must be a non-nil map (nil map writes panic).
	if ordersByCurrency == nil {
		t.Fatal("ordersByCurrency must be initialized")
	}
}
