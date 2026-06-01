package cofrinho

import "time"

// CofrinhoStatus represents the lifecycle of a cofrinho contribution.
type CofrinhoStatus string

const (
	StatusPendente   CofrinhoStatus = "PENDENTE"
	StatusConfirmado CofrinhoStatus = "CONFIRMADO"
	StatusExpirado   CofrinhoStatus = "EXPIRADO"
)

// CofrinhoContribuicao represents a gift/contribution in a cofrinho (piggy bank) module.
type CofrinhoContribuicao struct {
	ID               string
	EventoID         string
	GuestID          string
	Nome             string
	Mensagem         string
	Valor            int64 // in centavos
	Status           CofrinhoStatus
	PIXQRCode        string
	WebhookPaymentID string
	CriadoEm        time.Time
	UpdatedAt        time.Time
}

// IsOwner returns true if the given guestID owns this contribution.
func (c *CofrinhoContribuicao) IsOwner(guestID string) bool {
	return c.GuestID == guestID
}
