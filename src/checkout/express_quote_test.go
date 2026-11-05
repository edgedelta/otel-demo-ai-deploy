package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	pb "github.com/open-telemetry/opentelemetry-demo/src/checkout/genproto/oteldemo"
)

func TestRetryExpressQuoteSuccessDoesNotSleep(t *testing.T) {
	want := &pb.Money{CurrencyCode: "USD", Units: 5}
	calls, sleeps := 0, 0
	got := retryExpressQuote(
		func() (*pb.Money, error) { calls++; return want, nil },
		func(time.Duration) { sleeps++ },
		&pb.Money{CurrencyCode: "USD", Units: 1},
	)
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if calls != 1 || sleeps != 0 {
		t.Fatalf("calls=%d sleeps=%d, want 1 and 0", calls, sleeps)
	}
}

func TestRetryExpressQuoteFailureSleepsAndFallsBack(t *testing.T) {
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	fallback := &pb.Money{CurrencyCode: "USD", Units: 1}
	calls, sleeps := 0, 0
	got := retryExpressQuote(
		func() (*pb.Money, error) { calls++; return nil, errors.New("boom") },
		func(time.Duration) { sleeps++ },
		fallback,
	)
	if got != fallback || calls != expressQuoteAttempts || sleeps != expressQuoteAttempts {
		t.Fatalf("got=%v calls=%d sleeps=%d", got, calls, sleeps)
	}
}
