// Package tracing provides OpenTelemetry tracing helpers for use cases and adapters.
package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const defaultTracerName = "backend-go-role-organizado"

// StartSpan creates a new child span with the given name and tracer.
// Usage:
//
//	ctx, span := tracing.StartSpan(ctx, "usecase.auth.login")
//	defer span.End()
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := otel.Tracer(defaultTracerName)
	ctx, span := tracer.Start(ctx, name)
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}

// RecordError marks the span as error and records the exception.
func RecordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// Attribute helpers for common domain attributes.

func UserID(id string) attribute.KeyValue {
	return attribute.String("app.user.id", id)
}

func EventID(id string) attribute.KeyValue {
	return attribute.String("app.event.id", id)
}

func RateioID(id string) attribute.KeyValue {
	return attribute.String("app.rateio.id", id)
}

func DominioCategoria(cat string) attribute.KeyValue {
	return attribute.String("app.dominio.categoria", cat)
}
