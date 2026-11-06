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

type fakeCartClient struct {
	pb.CartServiceClient
	items []*pb.CartItem
}

func (f fakeCartClient) GetCart(context.Context, *pb.GetCartRequest, ...grpc.CallOption) (*pb.Cart, error) {
	return &pb.Cart{Items: f.items}, nil
}

type fakeProductCatalogClient struct{ pb.ProductCatalogServiceClient }

func (fakeProductCatalogClient) GetProduct(_ context.Context, in *pb.GetProductRequest, _ ...grpc.CallOption) (*pb.Product, error) {
	return &pb.Product{Id: in.GetId(), PriceUsd: &pb.Money{CurrencyCode: "USD", Units: 10}}, nil
}

type fakeCurrencyClient struct{ pb.CurrencyServiceClient }

func (fakeCurrencyClient) Convert(_ context.Context, in *pb.CurrencyConversionRequest, _ ...grpc.CallOption) (*pb.Money, error) {
	return in.GetFrom(), nil
}

func TestPrepareOrderAllowsMultipleProductLines(t *testing.T) {
	prevTracer := tracer
	tracer = noop.NewTracerProvider().Tracer("test")
	t.Cleanup(func() { tracer = prevTracer })

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"cost_usd":{"currency_code":"USD","units":5}}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	cs := &checkout{
		cartSvcClient: fakeCartClient{items: []*pb.CartItem{
			{ProductId: "p1", Quantity: 1},
			{ProductId: "p2", Quantity: 2},
			{ProductId: "p3", Quantity: 3},
		}},
		productCatalogSvcClient: fakeProductCatalogClient{},
		currencySvcClient:       fakeCurrencyClient{},
		shippingSvcAddr:         "http://shipping.test",
		httpClient:              client,
	}

	got, err := cs.prepareOrderItemsAndShippingQuoteFromCart(context.Background(), "user-1", "USD", &pb.Address{})
	if err != nil {
		t.Fatalf("multi-line cart rejected: %v", err)
	}
	if len(got.orderItems) != 3 {
		t.Errorf("got %d order items, want 3", len(got.orderItems))
	}
}
