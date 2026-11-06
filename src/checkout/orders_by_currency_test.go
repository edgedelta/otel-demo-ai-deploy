// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

func TestOrdersByCurrencyIncrementDoesNotPanic(t *testing.T) {
	ordersByCurrencyMu.Lock()
	defer ordersByCurrencyMu.Unlock()
	if ordersByCurrency == nil {
		t.Fatal("ordersByCurrency must be initialized")
	}
	before := ordersByCurrency["USD"]
	ordersByCurrency["USD"]++
	if got := ordersByCurrency["USD"]; got != before+1 {
		t.Fatalf("got %d, want %d", got, before+1)
	}
}
