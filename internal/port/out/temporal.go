// Package out defines the output-port (driven-side) interfaces.
package out

import "context"

// WorkflowStartOptions carries parameters for starting a Temporal workflow.
type WorkflowStartOptions struct {
	// WorkflowID is the deterministic workflow ID (from workflow ID helper functions).
	// Must match the Java backend workflow ID scheme for cross-backend signal delivery.
	WorkflowID string
	// TaskQueue is the Temporal task queue name.
	// Must match the task queue polled by the target worker (Go or Java during migration).
	TaskQueue string
}

// TemporalWorkflowStarter starts a new Temporal workflow instance.
// Implementations use the workflow ID for deduplication (at-most-once-per-ID semantics).
// A nil implementation or no-op should be used when Temporal is disabled.
type TemporalWorkflowStarter interface {
	StartWorkflow(ctx context.Context, opts WorkflowStartOptions, workflowFn interface{}, args ...interface{}) error
}

// TemporalWorkflowSignaler sends a named signal to a running Temporal workflow by ID.
// Used to interrupt long-lived workflows (e.g. signalling PaymentExpirationWorkflow
// when a payment webhook arrives from the PSP).
// A nil implementation or no-op should be used when Temporal is disabled.
type TemporalWorkflowSignaler interface {
	SignalWorkflow(ctx context.Context, workflowID, signal string, arg interface{}) error
}
