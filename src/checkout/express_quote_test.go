// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	pb "github.com/open-telemetry/opentelemetry-demo/src/checkout/genproto/oteldemo"
)

func newExpressQuoteTestCheckout(t *testing.T, handler http.HandlerFunc) *checkout {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prevLogger, prevTracer := logger, tracer
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	tracer = noop.NewTracerProvider().Tracer("test")
	t.Cleanup(func() { logger, tracer = prevLogger, prevTracer })
	return &checkout{shippingSvcAddr: srv.URL, httpClient: srv.Client()}
}

func TestQuoteExpressShippingReturnsOnFirstSuccess(t *testing.T) {
	var calls int32
	cs := newExpressQuoteTestCheckout(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		io.WriteString(w, `{"cost_usd":{"currency_code":"USD","units":7,"nanos":0}}`)
	})
	standard := &pb.Money{CurrencyCode: "USD", Units: 1}

	start := time.Now()
	got := cs.quoteExpressShipping(t.Context(), &pb.Address{}, nil, standard)
	elapsed := time.Since(start)

	if got.GetUnits() != 7 {
		t.Fatalf("expected express quote of 7 units, got %v", got)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("expected 1 quote call, got %d", n)
	}
	if elapsed >= expressQuoteBackoff {
		t.Fatalf("expected no backoff on success, took %v", elapsed)
	}
}

func TestQuoteExpressShippingNoSleepAfterLastAttempt(t *testing.T) {
	var calls int32
	cs := newExpressQuoteTestCheckout(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	standard := &pb.Money{CurrencyCode: "USD", Units: 1}

	start := time.Now()
	got := cs.quoteExpressShipping(t.Context(), &pb.Address{}, nil, standard)
	elapsed := time.Since(start)

	if got != standard {
		t.Fatalf("expected fallback to standard quote, got %v", got)
	}
	if n := atomic.LoadInt32(&calls); n != expressQuoteAttempts {
		t.Fatalf("expected %d quote calls, got %d", expressQuoteAttempts, n)
	}
	// Sleeps only between attempts: (attempts-1) backoffs, not attempts.
	max := time.Duration(expressQuoteAttempts)*expressQuoteBackoff - expressQuoteBackoff/2
	if elapsed < time.Duration(expressQuoteAttempts-1)*expressQuoteBackoff || elapsed >= max {
		t.Fatalf("unexpected elapsed %v, want in [%v, %v)", elapsed, time.Duration(expressQuoteAttempts-1)*expressQuoteBackoff, max)
	}
}
