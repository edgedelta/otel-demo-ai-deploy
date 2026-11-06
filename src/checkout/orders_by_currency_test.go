// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"sync"
	"testing"
)

func TestOrdersByCurrencyInitialisedAndConcurrentSafe(t *testing.T) {
	if ordersByCurrency == nil {
		t.Fatal("ordersByCurrency must be initialised")
	}
	ordersByCurrencyMu.Lock()
	ordersByCurrency["TEST"] = 0
	ordersByCurrencyMu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ordersByCurrencyMu.Lock()
			ordersByCurrency["TEST"]++
			ordersByCurrencyMu.Unlock()
		}()
	}
	wg.Wait()
	if got := ordersByCurrency["TEST"]; got != 50 {
		t.Fatalf("got %d, want 50", got)
	}
}
