// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	ld "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/ldcomponents"
	"github.com/launchdarkly/go-server-sdk/v7/testhelpers/ldtestdata"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	pb "github.com/open-telemetry/opentelemetry-demo/src/checkout/genproto/oteldemo"
)

func TestFlagEnabled(t *testing.T) {
	newClient := func(t *testing.T, value bool) *ld.LDClient {
		t.Helper()
		td := ldtestdata.DataSource()
		td.Update(td.Flag(expressShippingFlag).VariationForAll(value))
		client, err := ld.MakeCustomClient("test", ld.Config{DataSource: td, Events: ldcomponents.NoEvents()}, 0)
		if err != nil {
			t.Fatalf("make client: %v", err)
		}
		t.Cleanup(func() { client.Close() })
		return client
	}

	tests := []struct {
		desc   string
		client func(t *testing.T) *ld.LDClient
		key    string
		want   bool
	}{
		{desc: "no client", client: func(*testing.T) *ld.LDClient { return nil }, key: expressShippingFlag, want: false},
		{desc: "flag on", client: func(t *testing.T) *ld.LDClient { return newClient(t, true) }, key: expressShippingFlag, want: true},
		{desc: "flag off", client: func(t *testing.T) *ld.LDClient { return newClient(t, false) }, key: expressShippingFlag, want: false},
		{desc: "unknown flag", client: func(t *testing.T) *ld.LDClient { return newClient(t, true) }, key: "missing", want: false},
	}
	for _, tc := range tests {
		cs := &checkout{ld: tc.client(t)}
		if got := cs.flagEnabled(tc.key); got != tc.want {
			t.Errorf("%s: got: %v, want: %v", tc.desc, got, tc.want)
		}
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestSendOrderConfirmationHasBoundedTimeout(t *testing.T) {
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			deadline, ok := req.Context().Deadline()
			if !ok {
				return nil, fmt.Errorf("email request context has no deadline")
			}

			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > time.Second {
				return nil, fmt.Errorf("email request timeout = %v, want at most %v", remaining, time.Second)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	cs := &checkout{
		emailSvcAddr: "http://email.test",
		httpClient:   client,
	}

	err := cs.sendOrderConfirmation(context.Background(), "test@example.com", &pb.OrderResult{})
	if err != nil {
		t.Fatalf("sendOrderConfirmation() error = %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	cs := &checkout{}
	resp, err := cs.Check(context.Background(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error from Check: %v", err)
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("expected SERVING status, got %v", resp.GetStatus())
	}
}

func TestServerStartupAndGracefulStop(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on ephemeral port: %v", err)
	}

	srv := grpc.NewServer()
	svc := &checkout{}
	pb.RegisterCheckoutServiceServer(srv, svc)

	healthcheck := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthcheck)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect to test server: %v", err)
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check failed on test server: %v", err)
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("expected SERVING, got %v", resp.GetStatus())
	}

	stoppedCh := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stoppedCh)
	}()

	select {
	case <-stoppedCh:
		// Graceful stop completed
	case <-time.After(5 * time.Second):
		t.Fatal("GracefulStop timed out")
	}

	select {
	case err := <-serverErrCh:
		if err != nil && err != grpc.ErrServerStopped {
			t.Errorf("unexpected server exit error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit after GracefulStop")
	}
}

func TestSendToPostProcessorWithNilProducerDoesNotPanic(t *testing.T) {
	var logs bytes.Buffer
	prevLogger, prevTracer := logger, tracer
	logger = slog.New(slog.NewTextHandler(&logs, nil))
	tracer = noop.NewTracerProvider().Tracer("test")
	t.Cleanup(func() { logger, tracer = prevLogger, prevTracer })

	// Broker address configured but producer creation failed at startup.
	cs := &checkout{kafkaBrokerSvcAddr: "kafka:9092"}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("sendToPostProcessor panicked with nil producer: %v", r)
		}
	}()
	cs.sendToPostProcessor(context.Background(), &pb.OrderResult{OrderId: "order-1"})

	if !strings.Contains(logs.String(), "Kafka producer is not initialized") {
		t.Errorf("expected diagnostic log for nil producer, got: %q", logs.String())
	}
}

func TestQuoteExpressShipping(t *testing.T) {
	prevLogger, prevTracer := logger, tracer
	logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	tracer = noop.NewTracerProvider().Tracer("test")
	t.Cleanup(func() { logger, tracer = prevLogger, prevTracer })

	standard := &pb.Money{CurrencyCode: "USD", Units: 5}
	express := `{"cost_usd":{"currency_code":"USD","units":12}}`

	tests := []struct {
		desc      string
		fail      int // number of leading failing calls
		wantCalls int
		wantUnits int64
		maxTime   time.Duration
	}{
		{desc: "first attempt succeeds without sleeping", fail: 0, wantCalls: 1, wantUnits: 12, maxTime: expressQuoteBackoff / 2},
		{desc: "succeeds after one failure", fail: 1, wantCalls: 2, wantUnits: 12, maxTime: 2 * expressQuoteBackoff},
		{desc: "all attempts fail, no sleep after last", fail: expressQuoteAttempts, wantCalls: expressQuoteAttempts, wantUnits: 5, maxTime: time.Duration(expressQuoteAttempts)*expressQuoteBackoff - expressQuoteBackoff/2},
	}
	for _, tc := range tests {
		calls := 0
		cs := &checkout{
			shippingSvcAddr: "http://shipping.test",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				status, body := http.StatusOK, express
				if calls <= tc.fail {
					status, body = http.StatusInternalServerError, ""
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: req}, nil
			})},
		}
		start := time.Now()
		got := cs.quoteExpressShipping(context.Background(), &pb.Address{}, nil, standard)
		elapsed := time.Since(start)
		if calls != tc.wantCalls {
			t.Errorf("%s: calls = %d, want %d", tc.desc, calls, tc.wantCalls)
		}
		if got.GetUnits() != tc.wantUnits {
			t.Errorf("%s: units = %d, want %d", tc.desc, got.GetUnits(), tc.wantUnits)
		}
		if elapsed > tc.maxTime {
			t.Errorf("%s: took %v, want at most %v", tc.desc, elapsed, tc.maxTime)
		}
	}
}
