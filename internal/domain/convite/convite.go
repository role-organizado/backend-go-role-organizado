// Package convite holds the domain types and pure functions for the Convites
// (invitations) bounded context. This mirrors the Java backend's invite domain:
// participants, guests, approval items of type INVITE, refund tier evaluation,
// phone normalisation and channel selection for invite delivery.
//
// The package is dependency-free (no DB, no HTTP) so it can be unit-tested in
// isolation.
package convite

import (
	"regexp"
	"strings"
	"time"
)

// ---- Value-object enums ----

// TipoParticipante distinguishes between a GUEST (unregistered, lazy materialised)
// and a USER (registered usuário with an account).
type TipoParticipante string

const (
	TipoGuest TipoParticipante = "GUEST"
	TipoUser  TipoParticipante = "USER"
)

// ParticipantStatus reflects the lifecycle of a participation/invite.
type ParticipantStatus string

const (
	StatusPendente   ParticipantStatus = "PENDENTE"
	StatusConfirmado ParticipantStatus = "CONFIRMADO"
	StatusRecusado   ParticipantStatus = "RECUSADO"
	StatusCancelado  ParticipantStatus = "CANCELADO"
)

// Papel identifies a participant's role inside an event.
type Papel string

const (
	PapelOrganizador   Papel = "ORGANIZADOR"
	PapelCoOrganizador Papel = "CO_ORGANIZADOR"
	PapelConvidado     Papel = "CONVIDADO"
)

// ApprovalItemType identifies the kind of approval item; only INVITE is handled
// directly by this domain.
type ApprovalItemType string

const (
	ApprovalTypeInvite ApprovalItemType = "INVITE"
)

// ApprovalItemStatus reflects the lifecycle of an approval item.
type ApprovalItemStatus string

const (
	ApprovalStatusPending   ApprovalItemStatus = "PENDING"
	ApprovalStatusApproved  ApprovalItemStatus = "APPROVED"
	ApprovalStatusRejected  ApprovalItemStatus = "REJECTED"
	ApprovalStatusExpired   ApprovalItemStatus = "EXPIRED"
	ApprovalStatusCancelled ApprovalItemStatus = "CANCELLED"
)

// MaterializationStrategy controls how lazy-on-approval invites are resolved.
type MaterializationStrategy string

const (
	MaterializationLazyOnApproval MaterializationStrategy = "LAZY_ON_APPROVAL"
)

// CancellationCreditReason identifies why a participant credit was issued.
type CancellationCreditReason string

const (
	CreditReasonDesistencia CancellationCreditReason = "DESISTENCIA"
)

// ---- Entities ----

// Participant represents a participation document (collection: participants).
// Only the fields required by the convites domain are modelled; other fields
// are tolerated by the persistence layer when reading existing Java docs.
type Participant struct {
	ID                string
	EventoID          string
	UsuarioID         string
	TipoParticipante  TipoParticipante
	Papel             Papel
	Status            ParticipantStatus
	Nome              string
	Email             string
	Telefone          string
	GuestID           string
	DataResposta      *time.Time
	CriadoEm          time.Time
	AtualizadoEm      time.Time
}

// Guest represents a guest contact (collection: guests).
type Guest struct {
	ID                  string
	Nome                string
	Telefone            string
	Email               string
	EvoluidoParaUserID  string
	EvoluidoEm          *time.Time
	CriadoEm            time.Time
	AtualizadoEm        time.Time
}

// ApprovalItem represents an approval_items document (collection: approval_items).
// Only the INVITE-specific projection is modelled.
type ApprovalItem struct {
	ID                      string
	Type                    ApprovalItemType
	Status                  ApprovalItemStatus
	ApproverID              string
	EventID                 string
	TargetEntityID          string
	Metadata                map[string]any
	MaterializationStrategy MaterializationStrategy
	ExpiresAt               *time.Time
	ResolvedAt              *time.Time
	ResolvedBy              string
	ResolvedNote            string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// ---- Cancellation tier ----

// CancellationTier represents a single refund tier (days-before → refund %).
type CancellationTier struct {
	DiasMinimos     int     // inclusive lower bound: refund if daysBefore >= this
	RefundPercent   float64 // 0.0–1.0
	Label           string  // human label, e.g. "CANCELAMENTO ANTECIPADO"
}

// CancellationPolicy is an ordered list of tiers (descending DiasMinimos).
type CancellationPolicy struct {
	Chave string
	Tiers []CancellationTier
}

// DefaultGenericPolicy is the Java fallback (GENERICA_FLEXIVEL).
func DefaultGenericPolicy() CancellationPolicy {
	return CancellationPolicy{
		Chave: "GENERICA_FLEXIVEL",
		Tiers: []CancellationTier{
			{DiasMinimos: 30, RefundPercent: 1.0, Label: "CANCELAMENTO ANTECIPADO (>=30 dias)"},
			{DiasMinimos: 15, RefundPercent: 0.50, Label: "CANCELAMENTO PARCIAL (>=15 dias)"},
			{DiasMinimos: 0, RefundPercent: 0.0, Label: "CANCELAMENTO TARDIO (<15 dias)"},
		},
	}
}

// EvaluateRefund computes the refund cents and tier label for a desistência
// given the amount already paid (in cents) and the number of days remaining
// before the event. Returns (refundCents, percent, label).
func (p CancellationPolicy) EvaluateRefund(totalPaidCents int64, daysBefore int) (int64, float64, string) {
	for _, t := range p.Tiers {
		if daysBefore >= t.DiasMinimos {
			refund := int64(float64(totalPaidCents) * t.RefundPercent)
			return refund, t.RefundPercent, t.Label
		}
	}
	return 0, 0, "SEM_REEMBOLSO"
}

// ---- Channel & phone normalisation ----

// Canal identifies the delivery channel selected for an invite.
type Canal string

const (
	CanalWhatsApp              Canal = "WHATSAPP"
	CanalEmail                 Canal = "EMAIL"
	CanalWhatsAppFallbackEmail Canal = "WHATSAPP_FALLBACK_EMAIL"
)

// SelectCanal mirrors Java's channel-selection rule based on contact availability.
func SelectCanal(telefone, email string) Canal {
	hasPhone := strings.TrimSpace(telefone) != ""
	hasEmail := strings.TrimSpace(email) != ""
	switch {
	case hasPhone && hasEmail:
		return CanalWhatsAppFallbackEmail
	case hasPhone:
		return CanalWhatsApp
	case hasEmail:
		return CanalEmail
	default:
		return ""
	}
}

var nonNumericRegex = regexp.MustCompile(`[^0-9+]`)

// NormalizePhoneE164 normalises an arbitrary phone string to E.164 with default
// DDI 55 (Brasil). Returns empty string when the input is blank or has no digits.
func NormalizePhoneE164(raw string) string {
	if raw == "" {
		return ""
	}
	cleaned := nonNumericRegex.ReplaceAllString(raw, "")
	if cleaned == "" {
		return ""
	}
	// If the user already supplied +, keep it and strip duplicate symbols.
	if strings.HasPrefix(cleaned, "+") {
		digits := strings.TrimLeft(cleaned, "+")
		if digits == "" {
			return ""
		}
		return "+" + digits
	}
	// Otherwise, assume Brazilian DDI (55) and pad.
	// Avoid double-prefix if the number already starts with 55 and is long enough.
	if strings.HasPrefix(cleaned, "55") && len(cleaned) >= 12 {
		return "+" + cleaned
	}
	return "+55" + cleaned
}

// ---- Pure domain helpers ----

// IsTerminalParticipantStatus reports whether the status is one that no longer
// accepts public link transitions (CONFIRMADO/RECUSADO/CANCELADO).
func IsTerminalParticipantStatus(s ParticipantStatus) bool {
	switch s {
	case StatusConfirmado, StatusRecusado, StatusCancelado:
		return true
	default:
		return false
	}
}

// CanReopenInvite reports whether a previous invite ApprovalItem may be reopened.
// Java rule: only when previous is NOT APPROVED and NOT PENDING.
func CanReopenInvite(prev *ApprovalItem) bool {
	if prev == nil {
		return true
	}
	switch prev.Status {
	case ApprovalStatusApproved, ApprovalStatusPending:
		return false
	}
	return true
}

// DaysBefore returns how many integer days separate now from target, rounded down.
// Negative when target is in the past.
func DaysBefore(now, target time.Time) int {
	return int(target.Sub(now).Hours() / 24)
}
