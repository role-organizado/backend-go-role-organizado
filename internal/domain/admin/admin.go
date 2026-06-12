// Package admin holds the read models and value objects for the admin surface:
// dashboard metrics, feature flags, approval items, pending outbound aggregation,
// cancellation policies, and reconciliation report reads.
//
// These mirror the shapes returned by the Java admin controllers
// (AdminDashboardController, AdminDominiosController, AdminEventosController,
// AdminOperationsController, CancelamentoPoliciesAdminController) so the BFF and
// front-end keep parity during the Strangler-Fig migration.
package admin

import (
	"strings"
	"time"
)

// DashboardCounts holds the aggregate "big number" metrics for the admin dashboard.
type DashboardCounts struct {
	TotalUsuarios     int64
	TotalEventos      int64
	TotalDrafts       int64
	TotalPagamentos   int64
	TotalNotificacoes int64
}

// FeatureFlag mirrors a document in the feature_flags collection.
type FeatureFlag struct {
	ID           string
	Chave        string
	Nome         string
	Enabled      bool
	Descricao    string
	Categoria    string
	Metadata     map[string]any
	CriadoEm     string
	AtualizadoEm string
}

// FeatureFlagUpdate carries the mutable fields for an UpdateFeatureFlag call.
// Pointer fields distinguish "absent" (nil → leave untouched) from a zero value.
type FeatureFlagUpdate struct {
	Enabled   *bool
	Nome      *string
	Descricao *string
}

// ApprovalItem is a read model of an approval_items document
// (Java parity: ApprovalItemDTO).
type ApprovalItem struct {
	ID            string
	Tipo          string
	EventoID      string
	SolicitanteID string
	Status        string
	CriadoEm      any
}

// PendingOutboundItem aggregates the pending outbound requests of a single event.
type PendingOutboundItem struct {
	EventID            string
	EventName          string
	PendingCount       int
	PendingAmountCents int64
	OldestPendingAt    *time.Time
}

// PendingOutboundSummary is the response for the accounting/pending-outbound endpoint.
type PendingOutboundSummary struct {
	Timestamp               time.Time
	TotalEventsWithPending  int
	TotalPendingRequests    int
	TotalPendingAmountCents int64
	Items                   []PendingOutboundItem
}

// CancellationTier is one tier of a cancellation policy
// (Java parity: CancellationTier inside dominios.metadata.tiers).
type CancellationTier struct {
	TriggerType   string `json:"triggerType"`
	Threshold     int    `json:"threshold"`
	RefundPercent int    `json:"refundPercent"`
	Label         string `json:"label"`
}

// Valid mirrors Java's CancellationTier.isValid:
// triggerType ∈ {DAYS_BEFORE_EVENT, PHASE_AT_OR_AFTER}, threshold ≥ 0,
// refundPercent ∈ [0,100], label non-blank.
func (t CancellationTier) Valid() bool {
	switch t.TriggerType {
	case "DAYS_BEFORE_EVENT", "PHASE_AT_OR_AFTER":
	default:
		return false
	}
	if t.Threshold < 0 {
		return false
	}
	if t.RefundPercent < 0 || t.RefundPercent > 100 {
		return false
	}
	return strings.TrimSpace(t.Label) != ""
}

// ReconciliationReport is the read model for a reconciliation_reports document.
type ReconciliationReport struct {
	ID            string
	ReferenceDate string
	RunAt         time.Time
	CheckedCount  int64
	UpdatedCount  int64
	FailedCount   int64
	Updates       []ReconciliationUpdate
	Errors        []string
}

// ReconciliationUpdate is one transaction status change inside a report.
type ReconciliationUpdate struct {
	TransactionID  string
	PreviousStatus string
	NewStatus      string
	ProviderStatus string
	UpdatedAt      time.Time
}
