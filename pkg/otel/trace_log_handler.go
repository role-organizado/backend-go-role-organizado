package otel

import (
	"context"
	goslog "log/slog"

	"go.opentelemetry.io/otel/trace"
)

// traceContextHandler wraps an slog.Handler and injects trace_id and span_id
// from the active OTel span into every log record. This enables log-trace
// correlation directly in the JSON stdout output without requiring the OTel
// log bridge.
type traceContextHandler struct {
	inner goslog.Handler
}

// NewTraceContextHandler returns an slog.Handler that decorates inner with
// trace_id and span_id fields whenever a valid span is active in ctx.
func NewTraceContextHandler(inner goslog.Handler) goslog.Handler {
	return &traceContextHandler{inner: inner}
}

func (h *traceContextHandler) Enabled(ctx context.Context, level goslog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceContextHandler) Handle(ctx context.Context, r goslog.Record) error {
	span := trace.SpanFromContext(ctx)
	if sc := span.SpanContext(); sc.IsValid() {
		r.AddAttrs(
			goslog.String("trace_id", sc.TraceID().String()),
			goslog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *traceContextHandler) WithAttrs(attrs []goslog.Attr) goslog.Handler {
	return &traceContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceContextHandler) WithGroup(name string) goslog.Handler {
	return &traceContextHandler{inner: h.inner.WithGroup(name)}
}
