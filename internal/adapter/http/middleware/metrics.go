// Package middleware provides HTTP middleware for the Go backend.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const tracerName = "backend-go-role-organizado/http"

var (
	httpRequestsTotal   metric.Int64Counter
	httpRequestDuration metric.Float64Histogram
)

func initMetrics() {
	meter := otel.GetMeterProvider().Meter(tracerName)
	var err error

	httpRequestsTotal, err = meter.Int64Counter(
		"http.server.request.total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		// Non-fatal: metrics disabled gracefully
		return
	}

	httpRequestDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return
	}
}

func init() {
	initMetrics()
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Metrics returns middleware that records HTTP request metrics via OpenTelemetry.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)

		next.ServeHTTP(rw, r)

		duration := float64(time.Since(start).Milliseconds())
		status := strconv.Itoa(rw.statusCode)

		attrs := []attribute.KeyValue{
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.URL.Path),
			attribute.String("http.status_code", status),
		}

		if httpRequestsTotal != nil {
			httpRequestsTotal.Add(r.Context(), 1, metric.WithAttributes(attrs...))
		}
		if httpRequestDuration != nil {
			httpRequestDuration.Record(r.Context(), duration, metric.WithAttributes(attrs...))
		}
	})
}
