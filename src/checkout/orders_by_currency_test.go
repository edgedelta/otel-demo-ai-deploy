// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

func TestOrdersByCurrencyInitialized(t *testing.T) {
	if ordersByCurrency == nil {
		t.Fatal("ordersByCurrency must be initialized; writing to a nil map panics")
	}
	ordersByCurrencyMu.Lock()
	defer ordersByCurrencyMu.Unlock()
	ordersByCurrency["USD"]++
	if ordersByCurrency["USD"] < 1 {
		t.Fatalf("expected USD count >= 1, got %d", ordersByCurrency["USD"])
	}
}
