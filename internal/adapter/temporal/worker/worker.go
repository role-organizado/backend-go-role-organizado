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

// RegisterFinanceReconciliationWorker registers the FinanceReconciliation workflow
// with its native (Go) activities. Callers must build the activities via
// temporalactivity.NewFinanceReconciliationActivities(reconUC) with a fully
// wired ReconciliationUseCase.
func (r *Registry) RegisterFinanceReconciliationWorker(acts *temporalactivity.FinanceReconciliationActivities) {
	w := r.NewWorker(FinanceReconciliationQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.FinanceReconciliationWorkflow)
	w.RegisterActivity(acts)
}

// OverdueInstallmentQueue is the Temporal task queue for overdue installment workers.
const OverdueInstallmentQueue = "OVERDUE_INSTALLMENT_QUEUE"

// RegisterOverdueInstallmentWorker registers the OverdueInstallment workflow and activities.
func (r *Registry) RegisterOverdueInstallmentWorker(acts *temporalactivity.OverdueInstallmentActivities) {
	w := r.NewWorker(OverdueInstallmentQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.OverdueInstallmentWorkflow)
	w.RegisterActivity(acts)
}

// ─── Onda 3/4 native workflow workers ──────────────────────────────────────────

// RegisterParticipantLifecycleWorker registers the ParticipantLifecycleWorkflow
// and its activities on PARTICIPANT_LIFECYCLE_QUEUE.
func (r *Registry) RegisterParticipantLifecycleWorker(acts *temporalactivity.ParticipantLifecycleActivities) {
	w := r.NewWorker(temporalworkflow.ParticipantLifecycleTaskQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.ParticipantLifecycleWorkflow)
	w.RegisterActivity(acts)
}

// RegisterInviteLifecycleWorker registers the InviteLifecycleWorkflow and its
// activities on INVITE_LIFECYCLE_QUEUE.
func (r *Registry) RegisterInviteLifecycleWorker(acts *temporalactivity.InviteLifecycleActivities) {
	w := r.NewWorker(temporalworkflow.InviteLifecycleTaskQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.InviteLifecycleWorkflow)
	w.RegisterActivity(acts)
}

// RegisterOutboundExecutionWorker registers the OutboundExecutionWorkflow and its
// activities on OUTBOUND_EXECUTION_QUEUE.
func (r *Registry) RegisterOutboundExecutionWorker(acts *temporalactivity.OutboundExecutionActivities) {
	w := r.NewWorker(temporalworkflow.OutboundExecutionTaskQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.OutboundExecutionWorkflow)
	w.RegisterActivity(acts)
}

// RegisterEventLifecycleWorker registers the EventLifecycleWorkflow and its
// activities on EVENT_LIFECYCLE_QUEUE.
func (r *Registry) RegisterEventLifecycleWorker(acts *temporalactivity.EventLifecycleActivities) {
	w := r.NewWorker(temporalworkflow.EventLifecycleTaskQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.EventLifecycleWorkflow)
	w.RegisterActivity(acts)
}

// RegisterEventPublicationMonitoringWorker registers the
// EventPublicationMonitoringWorkflow and its activities on
// EVENT_PUBLICATION_MONITORING_QUEUE.
func (r *Registry) RegisterEventPublicationMonitoringWorker(acts *temporalactivity.EventPublicationMonitoringActivities) {
	w := r.NewWorker(temporalworkflow.EventPublicationMonitoringTaskQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.EventPublicationMonitoringWorkflow)
	w.RegisterActivity(acts)
}

// RegisterEventPublicationExecutionWorker registers the
// EventPublicationExecutionWorkflow and its activities on
// EVENT_PUBLICATION_EXECUTION_QUEUE.
func (r *Registry) RegisterEventPublicationExecutionWorker(acts *temporalactivity.EventPublicationExecutionActivities) {
	w := r.NewWorker(temporalworkflow.EventPublicationExecutionTaskQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.EventPublicationExecutionWorkflow)
	w.RegisterActivity(acts)
}
