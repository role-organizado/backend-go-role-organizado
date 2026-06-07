// Package workflow provides stable workflow ID constructors that match Java's
// TemporalWorkflowIds.java exactly. Every function here has a 1-to-1 counterpart
// in the Java implementation so that workflow IDs remain consistent during the
// Strangler Fig migration period.
package workflow

import "strings"

// EventPublicationPrimaryID returns the workflow ID for an EventPublicationExecution.
func EventPublicationPrimaryID(draftId string) string {
	return "event-publication-real-" + draftId
}

// OutboundPrimaryID returns the workflow ID for an OutboundExecution.
func OutboundPrimaryID(outboundRequestId string) string {
	return "outbound-real-" + outboundRequestId
}

// ParticipantLifecyclePrimaryID returns the workflow ID for a ParticipantLifecycle.
func ParticipantLifecyclePrimaryID(eventId, participantId string) string {
	return "participant-lifecycle-real-" + eventId + "-" + participantId
}

// PspReconciliationPrimaryID returns the workflow ID for a manual PspReconciliation run.
func PspReconciliationPrimaryID(scopeId, referenceDate string) string {
	return "psp-reconciliation-real-" + scopeId + "-" + referenceDate
}

// PaymentExpirationPrimaryID returns the workflow ID for a PaymentExpiration.
func PaymentExpirationPrimaryID(transactionId string) string {
	return "payment-expiration-real-" + transactionId
}

// FinanceReconciliationPrimaryID returns the workflow ID for a manual FinanceReconciliation run.
func FinanceReconciliationPrimaryID(referenceDate string) string {
	return "finance-reconciliation-real-" + referenceDate
}

// OverdueInstallmentPrimaryID returns the workflow ID for a manual OverdueInstallment run.
func OverdueInstallmentPrimaryID(referenceDate string) string {
	return "overdue-installment-real-" + referenceDate
}

// InviteLifecyclePrimaryID returns the workflow ID for an InviteLifecycle.
func InviteLifecyclePrimaryID(approvalId string) string {
	return "invite-lifecycle-real-" + approvalId
}

// AccountingSnapshotPrimaryID returns the workflow ID for an AccountingSnapshot.
// When correlationId is provided it is used directly as the discriminator key;
// otherwise the key is composed from dataInicio and dataFim (with "all"/"today"
// as fallbacks), matching the Java AccountingSnapshotWorkflowIds logic.
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

// NotificationFallbackPrimaryID returns the workflow ID for a NotificationFallback.
func NotificationFallbackPrimaryID(notificationId string) string {
	return "notification-fallback-real-" + notificationId
}

// PricingPspReviewPrimaryID returns the workflow ID for a manual PricingPspReview run.
func PricingPspReviewPrimaryID(referenceDate string) string {
	return "pricing-psp-review-real-" + referenceDate
}

// EventPublicationMonitoringPrimaryID returns the fixed singleton workflow ID
// for the continuous EventPublicationMonitoring loop.
func EventPublicationMonitoringPrimaryID() string {
	return "event-publication-monitoring-real"
}

// ParticipantRecalculationPrimaryID returns the workflow ID for a ParticipantRecalculation.
func ParticipantRecalculationPrimaryID(eventId string) string {
	return "participant-recalculation-real-" + eventId
}

// EventLifecyclePrimaryID returns the workflow ID for an EventLifecycle.
func EventLifecyclePrimaryID(eventoId string) string {
	return "event-lifecycle-real-" + eventoId
}

// PaymentConfirmationPrimaryID returns the workflow ID for a PaymentConfirmation.
// callbackType is lower-cased and defaults to "unknown" when empty.
func PaymentConfirmationPrimaryID(providerTransactionId, callbackType string) string {
	ct := callbackType
	if ct == "" {
		ct = "unknown"
	}
	return "payment-confirmation-real-" + providerTransactionId + "-" + strings.ToLower(ct)
}

// Scheduled workflow IDs — fixed identifiers for Temporal schedules that trigger
// daily batch workflows. These must match the Java TemporalScheduleInitializer
// constants so that the same schedule name is reused after Go cutover.
const (
	PspReconciliationScheduledID     = "psp-reconciliation-daily-workflow"
	FinanceReconciliationScheduledID = "finance-reconciliation-daily-workflow"
	OverdueInstallmentScheduledID    = "overdue-installment-daily-workflow"
	PricingPspReviewScheduledID      = "pricing-psp-review-daily-workflow"
)
