// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"

	pb "github.com/open-telemetry/opentelemetry-demo/src/checkout/genproto/oteldemo"
)

type fakeCart struct {
	pb.CartServiceClient
	items []*pb.CartItem
}

func (f fakeCart) GetCart(context.Context, *pb.GetCartRequest, ...grpc.CallOption) (*pb.Cart, error) {
	return &pb.Cart{Items: f.items}, nil
}

type fakeCatalog struct{ pb.ProductCatalogServiceClient }

func (fakeCatalog) GetProduct(_ context.Context, r *pb.GetProductRequest, _ ...grpc.CallOption) (*pb.Product, error) {
	return &pb.Product{Id: r.GetId(), PriceUsd: &pb.Money{CurrencyCode: "USD", Units: 10}}, nil
}

type fakeCurrency struct{ pb.CurrencyServiceClient }

func (fakeCurrency) Convert(_ context.Context, r *pb.CurrencyConversionRequest, _ ...grpc.CallOption) (*pb.Money, error) {
	return r.GetFrom(), nil
}

func TestPrepareOrderItemsMultiItemCart(t *testing.T) {
	tracer = noop.NewTracerProvider().Tracer("test")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"cost_usd":{"currency_code":"USD","units":5,"nanos":0}}`))
	}))
	defer srv.Close()

	items := []*pb.CartItem{
		{ProductId: "A", Quantity: 1},
		{ProductId: "B", Quantity: 2},
		{ProductId: "C", Quantity: 3},
	}
	cs := &checkout{
		shippingSvcAddr:         srv.URL,
		cartSvcClient:           fakeCart{items: items},
		productCatalogSvcClient: fakeCatalog{},
		currencySvcClient:       fakeCurrency{},
		httpClient:              srv.Client(),
	}

	out, err := cs.prepareOrderItemsAndShippingQuoteFromCart(context.Background(), "user", "USD", &pb.Address{})
	if err != nil {
		t.Fatalf("multi-item cart should succeed, got error: %v", err)
	}
	if len(out.orderItems) != len(items) {
		t.Fatalf("expected %d order items, got %d", len(items), len(out.orderItems))
	}
}
