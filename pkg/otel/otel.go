// Package otel initializes the OpenTelemetry SDK for the backend.
// It configures OTLP HTTP exporters for traces, metrics, and logs,
// using the same endpoints as the Java backend for consistent observability.
package otel

import (
	"context"
	"fmt"
	goslog "log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// teeHandler fans out slog records to multiple handlers simultaneously.
type teeHandler struct {
	handlers []goslog.Handler
}

func (t *teeHandler) Enabled(ctx context.Context, level goslog.Level) bool {
	for _, h := range t.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (t *teeHandler) Handle(ctx context.Context, r goslog.Record) error {
	for _, h := range t.handlers {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r)
		}
	}
	return nil
}

func (t *teeHandler) WithAttrs(attrs []goslog.Attr) goslog.Handler {
	hs := make([]goslog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return &teeHandler{handlers: hs}
}

func (t *teeHandler) WithGroup(name string) goslog.Handler {
	hs := make([]goslog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		hs[i] = h.WithGroup(name)
	}
	return &teeHandler{handlers: hs}
}

// NewTeeHandler returns an slog.Handler that writes to both the provided JSON handler
// (stdout) and the OTEL bridge simultaneously, so logs land in both places when OTEL
// is enabled.
func NewTeeHandler(lp *sdklog.LoggerProvider, jsonHandler goslog.Handler) goslog.Handler {
	otelHandler := otelslog.NewHandler("backend-go-role-organizado", otelslog.WithLoggerProvider(lp))
	return &teeHandler{handlers: []goslog.Handler{jsonHandler, otelHandler}}
}

// Config holds the OTel SDK configuration.
type Config struct {
	// OTLPEndpoint is the OTLP HTTP collector endpoint (e.g. "http://otel-staging.rolds.dev:4318").
	OTLPEndpoint string
	// ServiceName is the logical service name for all telemetry (e.g. "backend-go-role-organizado").
	ServiceName string
	// ServiceVersion is the deployed version of this service.
	ServiceVersion string
	// Environment is the deployment environment (e.g. "staging", "production").
	Environment string
}

// Providers holds references to all initialized SDK providers.
// Call Shutdown on program exit to flush and close exporters.
type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	LoggerProvider *sdklog.LoggerProvider
	MeterProvider  *sdkmetric.MeterProvider
}

// Shutdown shuts down all SDK providers, flushing pending telemetry.
// Should be deferred in main.go after Init.
func (p *Providers) Shutdown(ctx context.Context) error {
	var errs []error
	if err := p.TracerProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
	}
	if err := p.LoggerProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("logger shutdown: %w", err))
	}
	if err := p.MeterProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("meter shutdown: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("otel shutdown errors: %v", errs)
	}
	return nil
}

// Init initializes the OpenTelemetry SDK and sets global providers.
// Returns a Providers struct whose Shutdown method must be called on exit.
func Init(ctx context.Context, cfg Config) (*Providers, error) {
	res, err := newResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating otel resource: %w", err)
	}

	tp, err := newTracerProvider(ctx, cfg.OTLPEndpoint, res)
	if err != nil {
		return nil, fmt.Errorf("creating tracer provider: %w", err)
	}
	otel.SetTracerProvider(tp)

	lp, err := newLoggerProvider(ctx, cfg.OTLPEndpoint, res)
	if err != nil {
		return nil, fmt.Errorf("creating logger provider: %w", err)
	}
	global.SetLoggerProvider(lp)

	mp, err := newMeterProvider(ctx, cfg.OTLPEndpoint, res)
	if err != nil {
		return nil, fmt.Errorf("creating meter provider: %w", err)
	}
	otel.SetMeterProvider(mp)

	return &Providers{
		TracerProvider: tp,
		LoggerProvider: lp,
		MeterProvider:  mp,
	}, nil
}

// NewSlogHandler returns an slog.Handler that forwards log records to OTel.
// Use this as the handler for slog.SetDefault in main.go.
func NewSlogHandler(lp *sdklog.LoggerProvider) goslog.Handler {
	return otelslog.NewHandler("backend-go-role-organizado", otelslog.WithLoggerProvider(lp))
}

func newResource(cfg Config) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
}

func newTracerProvider(ctx context.Context, endpoint string, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	), nil
}

func newLoggerProvider(ctx context.Context, endpoint string, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(endpoint),
		otlploghttp.WithInsecure(),
		otlploghttp.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, err
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	), nil
}

func newMeterProvider(ctx context.Context, endpoint string, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(endpoint),
		otlpmetrichttp.WithInsecure(),
		otlpmetrichttp.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	), nil
}
