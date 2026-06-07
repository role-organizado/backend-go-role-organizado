// Package temporal provides the Temporal SDK client and adapters for workflow
// orchestration within the Rolê Organizado backend.
package temporal

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"

	"github.com/role-organizado/backend-go-role-organizado/internal/config"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// NewClient creates a new Temporal client from the provided config.
// Returns an error if the connection to the Temporal frontend cannot be established.
func NewClient(cfg config.TemporalConfig) (client.Client, error) {
	c, err := client.Dial(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("temporal: dial %s (namespace=%s): %w", cfg.HostPort, cfg.Namespace, err)
	}
	return c, nil
}

// ─── ClientWorkflowStarter ────────────────────────────────────────────────────

// ClientWorkflowStarter wraps a Temporal client and implements portout.TemporalWorkflowStarter.
type ClientWorkflowStarter struct {
	c client.Client
}

// NewClientWorkflowStarter creates a starter backed by a real Temporal client.
func NewClientWorkflowStarter(c client.Client) *ClientWorkflowStarter {
	return &ClientWorkflowStarter{c: c}
}

// StartWorkflow starts a new workflow execution or is a no-op if the workflow
// with the same ID is already running (ALLOW_DUPLICATE_FAILED_ONLY semantics).
func (s *ClientWorkflowStarter) StartWorkflow(ctx context.Context, opts portout.WorkflowStartOptions, workflowFn interface{}, args ...interface{}) error {
	wo := client.StartWorkflowOptions{
		ID:        opts.WorkflowID,
		TaskQueue: opts.TaskQueue,
	}
	_, err := s.c.ExecuteWorkflow(ctx, wo, workflowFn, args...)
	if err != nil {
		return fmt.Errorf("start workflow %s on %s: %w", opts.WorkflowID, opts.TaskQueue, err)
	}
	slog.DebugContext(ctx, "temporal: started workflow", "workflowID", opts.WorkflowID, "taskQueue", opts.TaskQueue)
	return nil
}

// ─── ClientWorkflowSignaler ───────────────────────────────────────────────────

// ClientWorkflowSignaler wraps a Temporal client and implements portout.TemporalWorkflowSignaler.
type ClientWorkflowSignaler struct {
	c client.Client
}

// NewClientWorkflowSignaler creates a signaler backed by a real Temporal client.
func NewClientWorkflowSignaler(c client.Client) *ClientWorkflowSignaler {
	return &ClientWorkflowSignaler{c: c}
}

// SignalWorkflow sends a named signal to the workflow identified by workflowID.
// The run ID is intentionally left empty (latest run).
func (s *ClientWorkflowSignaler) SignalWorkflow(ctx context.Context, workflowID, signal string, arg interface{}) error {
	err := s.c.SignalWorkflow(ctx, workflowID, "", signal, arg)
	if err != nil {
		// Non-fatal: if the workflow already completed, the signal is irrelevant.
		// Log at WARN level so ops can investigate if the pattern becomes frequent.
		slog.WarnContext(ctx, "temporal: signal workflow failed", "workflowID", workflowID, "signal", signal, "error", err)
		return fmt.Errorf("signal workflow %s (%s): %w", workflowID, signal, err)
	}
	slog.DebugContext(ctx, "temporal: signalled workflow", "workflowID", workflowID, "signal", signal)
	return nil
}

// ─── No-op implementations ────────────────────────────────────────────────────

// NoopWorkflowStarter discards all workflow starts.
// Used when TEMPORAL_WORKER_ENABLED=false to prevent dependency on a live Temporal cluster.
type NoopWorkflowStarter struct{}

// StartWorkflow is a no-op that logs at DEBUG level and returns nil.
func (n *NoopWorkflowStarter) StartWorkflow(ctx context.Context, opts portout.WorkflowStartOptions, _ interface{}, _ ...interface{}) error {
	slog.DebugContext(ctx, "temporal: noop start workflow (worker disabled)", "workflowID", opts.WorkflowID)
	return nil
}

// NoopWorkflowSignaler discards all workflow signals.
// Used when TEMPORAL_WORKER_ENABLED=false.
type NoopWorkflowSignaler struct{}

// SignalWorkflow is a no-op that logs at DEBUG level and returns nil.
func (n *NoopWorkflowSignaler) SignalWorkflow(ctx context.Context, workflowID, signal string, _ interface{}) error {
	slog.DebugContext(ctx, "temporal: noop signal workflow (worker disabled)", "workflowID", workflowID, "signal", signal)
	return nil
}

// Compile-time interface assertions.
var _ portout.TemporalWorkflowStarter = (*ClientWorkflowStarter)(nil)
var _ portout.TemporalWorkflowStarter = (*NoopWorkflowStarter)(nil)
var _ portout.TemporalWorkflowSignaler = (*ClientWorkflowSignaler)(nil)
var _ portout.TemporalWorkflowSignaler = (*NoopWorkflowSignaler)(nil)
