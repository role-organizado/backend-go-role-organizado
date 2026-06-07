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

// RegisterPricingPspReviewWorker registers the PricingPspReview workflow and activity
// on the PRICING_PSP_REVIEW_QUEUE task queue.
func (r *Registry) RegisterPricingPspReviewWorker(act *temporalactivity.PricingPspReviewActivity) {
	w := r.NewWorker("PRICING_PSP_REVIEW_QUEUE", sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.PricingPspReviewWorkflow)
	w.RegisterActivity(act)
}

// FinanceReconciliationQueue is the Temporal task queue for finance reconciliation workers.
const FinanceReconciliationQueue = "FINANCE_RECONCILIATION_QUEUE"

// RegisterFinanceReconciliationWorker registers the FinanceReconciliation workflow and
// creates its activities backed by the Java backend URL (Strangler Fig bridge).
func (r *Registry) RegisterFinanceReconciliationWorker(javaBackendURL string) {
	activities := temporalactivity.NewFinanceReconciliationActivities(javaBackendURL)
	w := r.NewWorker(FinanceReconciliationQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.FinanceReconciliationWorkflow)
	w.RegisterActivity(activities)
}

// OverdueInstallmentQueue is the Temporal task queue for overdue installment workers.
const OverdueInstallmentQueue = "OVERDUE_INSTALLMENT_QUEUE"

// RegisterOverdueInstallmentWorker registers the OverdueInstallment workflow and activities.
func (r *Registry) RegisterOverdueInstallmentWorker(acts *temporalactivity.OverdueInstallmentActivities) {
	w := r.NewWorker(OverdueInstallmentQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.OverdueInstallmentWorkflow)
	w.RegisterActivity(acts)
}
