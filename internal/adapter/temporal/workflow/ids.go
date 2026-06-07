// Package workflow contains Temporal workflow definitions and ID helpers
// for the Rolê Organizado backend (Go Strangler Fig implementation).
//
// WorkflowId naming mirrors Java's TemporalWorkflowIds exactly so that
// signal-by-workflowId, signal-with-start, and idempotency checks work
// identically across the two runtimes. Any drift between Go and Java IDs
// will break production signals → this is guarded by TestWorkflowIdParity.
package workflow

import "strings"

// ── Primary workflow IDs ─────────────────────────────────────────────────────

// EventPublicationPrimaryID returns the workflowId for an event publication execution.
func EventPublicationPrimaryID(draftID string) string {
	return "event-publication-real-" + draftID
}

// OutboundPrimaryID returns the workflowId for an outbound execution workflow.
func OutboundPrimaryID(outboundRequestID string) string {
	return "outbound-real-" + outboundRequestID
}

// ParticipantLifecyclePrimaryID returns the workflowId for a participant lifecycle workflow.
func ParticipantLifecyclePrimaryID(eventID, participantID string) string {
	return "participant-lifecycle-real-" + eventID + "-" + participantID
}

// PspReconciliationPrimaryID returns the workflowId for a PSP reconciliation workflow.
func PspReconciliationPrimaryID(scopeID, referenceDate string) string {
	return "psp-reconciliation-real-" + scopeID + "-" + referenceDate
}

// PaymentExpirationPrimaryID returns the workflowId for a payment expiration workflow.
func PaymentExpirationPrimaryID(transactionID string) string {
	return "payment-expiration-real-" + transactionID
}

// FinanceReconciliationPrimaryID returns the workflowId for a finance reconciliation workflow.
func FinanceReconciliationPrimaryID(referenceDate string) string {
	return "finance-reconciliation-real-" + referenceDate
}

// OverdueInstallmentPrimaryID returns the workflowId for an overdue installment workflow.
func OverdueInstallmentPrimaryID(referenceDate string) string {
	return "overdue-installment-real-" + referenceDate
}

// InviteLifecyclePrimaryID returns the workflowId for an invite lifecycle workflow.
func InviteLifecyclePrimaryID(approvalID string) string {
	return "invite-lifecycle-real-" + approvalID
}

// AccountingSnapshotPrimaryID returns the workflowId for an accounting snapshot workflow.
// Logic mirrors Java: prefer correlationId, then dataInicio+"_"+dataFim (with "all"/"today" defaults).
func AccountingSnapshotPrimaryID(dataInicio, dataFim, correlationID string) string {
	key := correlationID
	if key == "" {
		start := dataInicio
		if start == "" {
			start = "all"
		}
		end := dataFim
		if end == "" {
			end = "today"
		}
		key = start + "_" + end
	}
	return "accounting-snapshot-real-" + key
}

// NotificationFallbackPrimaryID returns the workflowId for a notification fallback workflow.
func NotificationFallbackPrimaryID(notificationID string) string {
	return "notification-fallback-real-" + notificationID
}

// PricingPspReviewPrimaryID returns the workflowId for a pricing PSP review workflow.
func PricingPspReviewPrimaryID(referenceDate string) string {
	return "pricing-psp-review-real-" + referenceDate
}

// EventPublicationMonitoringPrimaryID returns the singleton workflowId for the
// event publication monitoring workflow (long-running, continue-as-new).
func EventPublicationMonitoringPrimaryID() string {
	return "event-publication-monitoring-real"
}

// ParticipantRecalculationPrimaryID returns the workflowId for a participant
// installment recalculation workflow.
func ParticipantRecalculationPrimaryID(eventID string) string {
	return "participant-recalculation-real-" + eventID
}

// EventLifecyclePrimaryID returns the workflowId for an event lifecycle workflow.
func EventLifecyclePrimaryID(eventoID string) string {
	return "event-lifecycle-real-" + eventoID
}

// PaymentConfirmationPrimaryID returns the workflowId for a payment confirmation workflow.
// callbackType is lower-cased; empty string maps to "unknown" (mirrors Java null-check).
func PaymentConfirmationPrimaryID(providerTransactionID, callbackType string) string {
	ct := callbackType
	if ct == "" {
		ct = "unknown"
	} else {
		ct = strings.ToLower(ct)
	}
	return "payment-confirmation-real-" + providerTransactionID + "-" + ct
}

// ── Scheduled workflow IDs (constants) ───────────────────────────────────────
// Started by Temporal Schedules. Fixed IDs ensure only one execution is open
// per schedule at a time. Must be identical to Java's TemporalWorkflowIds constants.

const (
	// PspReconciliationScheduledID is the fixed workflowId for the daily PSP reconciliation schedule.
	PspReconciliationScheduledID = "psp-reconciliation-daily-workflow"

	// FinanceReconciliationScheduledID is the fixed workflowId for the daily finance reconciliation schedule.
	FinanceReconciliationScheduledID = "finance-reconciliation-daily-workflow"

	// OverdueInstallmentScheduledID is the fixed workflowId for the daily overdue installment schedule.
	OverdueInstallmentScheduledID = "overdue-installment-daily-workflow"

	// PricingPspReviewScheduledID is the fixed workflowId for the daily pricing PSP review schedule.
	PricingPspReviewScheduledID = "pricing-psp-review-daily-workflow"
)
