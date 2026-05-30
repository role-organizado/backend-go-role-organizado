package tracing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/role-organizado/backend-go-role-organizado/pkg/tracing"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestStartSpan_ReturnsContextAndSpan(t *testing.T) {
	ctx, span := tracing.StartSpan(context.Background(), "test.span")
	defer span.End()

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
}

func TestStartSpan_WithAttributes(t *testing.T) {
	ctx, span := tracing.StartSpan(context.Background(), "test.span.attrs",
		tracing.UserID("user-123"),
		tracing.EventID("event-456"),
	)
	defer span.End()

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
}

func TestRecordError_MarksSpanAsError(t *testing.T) {
	_, span := tracing.StartSpan(context.Background(), "test.error.span")
	defer span.End()

	// Should not panic
	tracing.RecordError(span, errors.New("test error"))
}

func TestAttributeHelpers(t *testing.T) {
	tests := []struct {
		name     string
		attr     attribute.KeyValue
		wantKey  string
		wantVal  string
	}{
		{"UserID", tracing.UserID("u1"), "app.user.id", "u1"},
		{"EventID", tracing.EventID("e1"), "app.event.id", "e1"},
		{"RateioID", tracing.RateioID("r1"), "app.rateio.id", "r1"},
		{"DominioCategoria", tracing.DominioCategoria("tipo_evento"), "app.dominio.categoria", "tipo_evento"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantKey, string(tt.attr.Key))
			assert.Equal(t, tt.wantVal, tt.attr.Value.AsString())
		})
	}
}
