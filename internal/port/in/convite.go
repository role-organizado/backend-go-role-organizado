package in

import (
	"context"
	"time"
)

// ============================================================
// Convites domain — use case interfaces (input ports).
// Mirrors Java's ConviteController + ApprovalController(reopen)
// + DesistenciaController + AdminEventosController(reenviar-todos).
// ============================================================

// ConviteResponse is the response returned to API clients describing a convite
// in its current state.
type ConviteResponse struct {
	ParticipantID     string     `json:"participantId"`
	Status            string     `json:"status"`
	EventoID          string     `json:"eventoId"`
	EventoNome        string     `json:"eventoNome,omitempty"`
	EventoData        *time.Time `json:"eventoData,omitempty"`
	EventoLocal       string     `json:"eventoLocal,omitempty"`
	EventoDescricao   string     `json:"eventoDescricao,omitempty"`
	OrganizadorNome   string     `json:"organizadorNome,omitempty"`
	ConvidadoNome     string     `json:"convidadoNome,omitempty"`
	ConvidadoEmail    string     `json:"convidadoEmail,omitempty"`
	ConvidadoTelefone string     `json:"convidadoTelefone,omitempty"`
	TipoParticipante  string     `json:"tipoParticipante,omitempty"`
	DataResposta      *time.Time `json:"dataResposta,omitempty"`
	EventoPassado     bool       `json:"eventoPassado"`
	Mensagem          string     `json:"mensagem,omitempty"`
}

// BuscarConviteUseCase looks up a convite by participantId, with the
// LAZY_ON_APPROVAL fallback to approval_items.
type BuscarConviteUseCase interface {
	Execute(ctx context.Context, participantID string) (*ConviteResponse, error)
}

// EnviarConviteInput holds the parameters to dispatch an invite.
type EnviarConviteInput struct {
	ParticipantID  string
	ForcarReenvio  bool
	OrganizadorID  string
}

// EnviarConviteResponse mirrors Java's EnviarConviteResponseDTO.
type EnviarConviteResponse struct {
	ParticipantID string `json:"participantId"`
	Aceito        bool   `json:"aceito"`
	MessageID     string `json:"messageId,omitempty"`
	Canal         string `json:"canal,omitempty"`
	Mensagem      string `json:"mensagem,omitempty"`
}

// EnviarConviteUseCase enqueues an invite delivery (SQS in production).
type EnviarConviteUseCase interface {
	Execute(ctx context.Context, in EnviarConviteInput) (*EnviarConviteResponse, error)
}

// ConfirmarConviteUseCase records the participant accepting the invite.
type ConfirmarConviteUseCase interface {
	Execute(ctx context.Context, participantID string) (*ConviteResponse, error)
}

// RecusarConviteUseCase records the participant declining the invite.
type RecusarConviteUseCase interface {
	Execute(ctx context.Context, participantID string) (*ConviteResponse, error)
}

// DesistenciaResult mirrors Java's DesistenciaResult record.
type DesistenciaResult struct {
	Status           string `json:"status"`
	TotalPagoCents   int64  `json:"totalPagoCents"`
	RefundAmountCents int64 `json:"refundAmountCents"`
	RefundPercent    float64 `json:"refundPercent"`
	TierLabel        string `json:"tierLabel,omitempty"`
	PoliticaAplicada string `json:"politicaAplicada,omitempty"`
	CreditID         string `json:"creditId,omitempty"`
}

// DesistirEventoUseCase confirms or previews a withdrawal from an event.
type DesistirEventoUseCase interface {
	Execute(ctx context.Context, participantID, userID string) (*DesistenciaResult, error)
	Preview(ctx context.Context, participantID, userID string) (*DesistenciaResult, error)
}

// ApprovalItemResponse describes the freshly created INVITE approval item.
type ApprovalItemResponse struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	Status         string         `json:"status"`
	EventID        string         `json:"eventId"`
	TargetEntityID string         `json:"targetEntityId"`
	ApproverID     string         `json:"approverId,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

// ReabrirInviteApprovalUseCase reopens an INVITE for a USER-type participant.
type ReabrirInviteApprovalUseCase interface {
	Execute(ctx context.Context, participantID, requesterID string) (*ApprovalItemResponse, error)
}

// EventoActionResponse mirrors Java's EventoActionResponseDTO.
type EventoActionResponse struct {
	EventoID string `json:"eventoId"`
	Acao     string `json:"acao"`
	Mensagem string `json:"mensagem"`
	Total    int64  `json:"total,omitempty"`
}

// ReenviarConvitesMassaAdminUseCase resends all invites for an event (admin only).
type ReenviarConvitesMassaAdminUseCase interface {
	Execute(ctx context.Context, eventoID, adminID string) (*EventoActionResponse, error)
}
