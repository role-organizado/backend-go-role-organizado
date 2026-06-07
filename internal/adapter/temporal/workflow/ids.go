// Package workflow provides Temporal workflow definitions and ID helpers for the
// Rolê Organizado backend.
//
// CRITICAL: Workflow ID functions must have EXACT parity with Java TemporalWorkflowIds.java.
// Divergence breaks signal-by-workflowId in production — e.g., a PIX payment webhook from
// the Java backend would fail to signal the Go PaymentExpirationWorkflow if IDs differ.
package workflow

import "strings"

// ─── Workflow ID constructors ─────────────────────────────────────────────────

// EventPublicationPrimaryID returns the workflow ID for an event publication workflow.
func EventPublicationPrimaryID(draftId string) string {
	return "event-publication-real-" + draftId
}

// OutboundPrimaryID returns the workflow ID for an outbound request workflow.
func OutboundPrimaryID(outboundRequestId string) string {
	return "outbound-real-" + outboundRequestId
}

// ParticipantLifecyclePrimaryID returns the workflow ID for a participant lifecycle workflow.
func ParticipantLifecyclePrimaryID(eventId, participantId string) string {
	return "participant-lifecycle-real-" + eventId + "-" + participantId
}

// PspReconciliationPrimaryID returns the workflow ID for a PSP reconciliation workflow.
func PspReconciliationPrimaryID(scopeId, referenceDate string) string {
	return "psp-reconciliation-real-" + scopeId + "-" + referenceDate
}

// PaymentExpirationPrimaryID returns the workflow ID for a payment expiration workflow.
// This ID MUST match the Java backend's PaymentExpirationWorkflow ID scheme so that
// signals from the Java webhook handler reach the Go worker during migration.
func PaymentExpirationPrimaryID(transactionId string) string {
	return "payment-expiration-real-" + transactionId
}

// FinanceReconciliationPrimaryID returns the workflow ID for a finance reconciliation workflow.
func FinanceReconciliationPrimaryID(referenceDate string) string {
	return "finance-reconciliation-real-" + referenceDate
}

// OverdueInstallmentPrimaryID returns the workflow ID for an overdue installment workflow.
func OverdueInstallmentPrimaryID(referenceDate string) string {
	return "overdue-installment-real-" + referenceDate
}

// InviteLifecyclePrimaryID returns the workflow ID for an invite lifecycle workflow.
func InviteLifecyclePrimaryID(approvalId string) string {
	return "invite-lifecycle-real-" + approvalId
}

// AccountingSnapshotPrimaryID returns the workflow ID for an accounting snapshot workflow.
// When correlationId is provided it is used as the key; otherwise a key is derived from
// the date range, matching the Java AccountingSnapshotWorkflow logic.
func AccountingSnapshotPrimaryID(dataInicio, dataFim, correlationId string) string {
	var key string
	if correlationId != "" {
		key = correlationId
	} else {
		di := dataInicio
		if di == "" {
			di = "all"
		}
		df := dataFim
		if df == "" {
			df = "today"
		}
		key = di + "_" + df
	}
	return "accounting-snapshot-real-" + key
}

// NotificationFallbackPrimaryID returns the workflow ID for a notification fallback workflow.
func NotificationFallbackPrimaryID(notificationId string) string {
	return "notification-fallback-real-" + notificationId
}

// PricingPspReviewPrimaryID returns the workflow ID for a pricing PSP review workflow.
func PricingPspReviewPrimaryID(referenceDate string) string {
	return "pricing-psp-review-real-" + referenceDate
}

// EventPublicationMonitoringPrimaryID returns the fixed workflow ID for the event
// publication monitoring workflow (singleton).
func EventPublicationMonitoringPrimaryID() string {
	return "event-publication-monitoring-real"
}

// ParticipantRecalculationPrimaryID returns the workflow ID for a participant
// recalculation workflow.
func ParticipantRecalculationPrimaryID(eventId string) string {
	return "participant-recalculation-real-" + eventId
}

// EventLifecyclePrimaryID returns the workflow ID for an event lifecycle workflow.
func EventLifecyclePrimaryID(eventoId string) string {
	return "event-lifecycle-real-" + eventoId
}

// PaymentConfirmationPrimaryID returns the workflow ID for a payment confirmation workflow.
// callbackType is lowercased for consistent ID generation.
func PaymentConfirmationPrimaryID(providerTransactionId, callbackType string) string {
	ct := callbackType
	if ct == "" {
		ct = "unknown"
	}
	return "payment-confirmation-real-" + providerTransactionId + "-" + strings.ToLower(ct)
}

// ─── Scheduled workflow IDs ───────────────────────────────────────────────────
// These constants correspond to Temporal Schedule IDs managed by the Java backend.
// The Go worker MUST use the same IDs to avoid creating duplicate schedules.

const (
	// PspReconciliationScheduledID is the Temporal Schedule ID for the daily PSP
	// reconciliation workflow. Matches Java app.temporal.schedules.psp-reconciliation.
	PspReconciliationScheduledID = "psp-reconciliation-daily-workflow"

	// FinanceReconciliationScheduledID is the Temporal Schedule ID for the daily
	// finance reconciliation workflow.
	FinanceReconciliationScheduledID = "finance-reconciliation-daily-workflow"

	// OverdueInstallmentScheduledID is the Temporal Schedule ID for the daily
	// overdue installment processing workflow.
	OverdueInstallmentScheduledID = "overdue-installment-daily-workflow"

	// PricingPspReviewScheduledID is the Temporal Schedule ID for the daily pricing
	// PSP review workflow.
	PricingPspReviewScheduledID = "pricing-psp-review-daily-workflow"
)

// ─── Task queues ──────────────────────────────────────────────────────────────

const (
	// PaymentTaskQueue is the Temporal task queue for all payment-related workflows.
	//
	// IMPORTANT — Strangler Fig compatibility:
	// During migration, both Go and Java workers MUST poll this same task queue so
	// that signals sent by the Java webhook handler reach the Go PaymentExpirationWorkflow
	// (and vice versa). If the Java backend uses a different queue name, update this
	// constant and redeploy both backends simultaneously.
	//
	// Java reference: search for PAYMENT_TASK_QUEUE or @Worker("payment-queue") in
	// the Java PaymentWorkflowConfiguration. If incompatible, use a signalling proxy:
	// Java sends signal → proxy reads Java queue → forwards to Go queue.
	PaymentTaskQueue = "payment-queue"

	// ReconciliationTaskQueue is the Temporal task queue for reconciliation workflows.
	ReconciliationTaskQueue = "reconciliation-queue"
// Package workflow contains Temporal workflow definitions and ID helpers.
// PricingPspReviewPrimaryID returns the deterministic workflow ID for a
// manual/triggered PricingPspReview run, scoped to a specific reference date.
// Example: pricing-psp-review-real-2026-06-06
)
