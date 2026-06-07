package worker

import (
	sdkworker "go.temporal.io/sdk/worker"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	temporalworkflow "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// RegisterPaymentWorker creates a worker on the payment task queue and registers
// all payment-related workflows and activities.
//
// Task queue: PaymentTaskQueue ("payment-queue") — must match the Java backend
// during Strangler Fig migration for cross-backend signal delivery.
func (r *Registry) RegisterPaymentWorker(acts *temporalactivity.PaymentActivities) {
	w := r.NewWorker(temporalworkflow.PaymentTaskQueue, sdkworker.Options{})

	// Workflows
	w.RegisterWorkflow(temporalworkflow.PaymentExpirationWorkflow)
	w.RegisterWorkflow(temporalworkflow.PaymentConfirmationWorkflow)

	// Activities — register the struct so all exported methods are available.
	w.RegisterActivity(acts)
}

// RegisterReconciliationWorker creates a worker on the reconciliation task queue
// and registers the ReconciliationWorkflow and its activities.
func (r *Registry) RegisterReconciliationWorker(acts *temporalactivity.PaymentActivities) {
	w := r.NewWorker(temporalworkflow.ReconciliationTaskQueue, sdkworker.Options{})

	// Workflows
	w.RegisterWorkflow(temporalworkflow.ReconciliationWorkflow)

	// Activities
	w.RegisterActivity(acts)
}

// RegisterSandboxWorker registers the SandboxWorkflow POC on SANDBOX_QUEUE.
func (r *Registry) RegisterSandboxWorker(act *temporalactivity.SandboxActivity) {
	w := r.NewWorker("SANDBOX_QUEUE", sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.SandboxWorkflow)
	w.RegisterActivity(act)
}
