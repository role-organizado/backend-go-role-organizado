package otel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/sdk/resource"
)

func TestEndpointHostPort(t *testing.T) {
	cases := map[string]string{
		"http://10.11.12.74:4318":     "10.11.12.74:4318",
		"https://collector:4318":      "collector:4318",
		"otel-staging.rolds.dev:4318": "otel-staging.rolds.dev:4318",
		"localhost:4318":              "localhost:4318",
	}
	for in, want := range cases {
		if got := endpointHostPort(in); got != want {
			t.Errorf("endpointHostPort(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestProvidersBuildWithSchemedEndpoint guards against the regression where a
// full-URL endpoint produced an invalid URL ("http://http:%2F%2F...") and made
// otel init fail, crash-looping the service.
func TestProvidersBuildWithSchemedEndpoint(t *testing.T) {
	ctx := context.Background()
	res := resource.Default()
	const endpoint = "http://10.11.12.74:4318"

	if _, err := newTracerProvider(ctx, endpoint, res); err != nil {
		t.Fatalf("newTracerProvider: %v", err)
	}
	if _, err := newLoggerProvider(ctx, endpoint, res); err != nil {
		t.Fatalf("newLoggerProvider: %v", err)
	}
	if _, err := newMeterProvider(ctx, endpoint, res); err != nil {
		t.Fatalf("newMeterProvider: %v", err)
	}
}
