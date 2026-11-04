package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	pb "github.com/open-telemetry/opentelemetry-demo/src/checkout/genproto/oteldemo"
)

func setupExpressTest(t *testing.T, handler http.HandlerFunc) *checkout {
	t.Helper()
	logger = slog.Default()
	tracer = noop.NewTracerProvider().Tracer("test")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &checkout{shippingSvcAddr: srv.URL, httpClient: srv.Client()}
}

func TestQuoteExpressShippingSuccessDoesNotSleep(t *testing.T) {
	var calls int32
	cs := setupExpressTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(`{"cost_usd":{"currency_code":"USD","units":7,"nanos":0}}`))
	})
	start := time.Now()
	got := cs.quoteExpressShipping(context.Background(), &pb.Address{}, nil, &pb.Money{Units: 1})
	if got.GetUnits() != 7 {
		t.Fatalf("expected express quote 7, got %d", got.GetUnits())
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("expected 1 shipping call on success, got %d", n)
	}
	if d := time.Since(start); d >= expressQuoteBackoff {
		t.Fatalf("successful quote should not sleep, took %v", d)
	}
}

func TestQuoteExpressShippingFallsBackToStandard(t *testing.T) {
	var calls int32
	cs := setupExpressTest(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	got := cs.quoteExpressShipping(context.Background(), &pb.Address{}, nil, &pb.Money{Units: 1})
	if got.GetUnits() != 1 {
		t.Fatalf("expected standard fallback 1, got %d", got.GetUnits())
	}
	if n := atomic.LoadInt32(&calls); n != expressQuoteAttempts {
		t.Fatalf("expected %d attempts, got %d", expressQuoteAttempts, n)
	}
}
