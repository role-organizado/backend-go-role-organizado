package payment

import "time"

// SavedCard represents a user's saved credit card for payment.
type SavedCard struct {
	ID          string
	UserID      string
	LastFour    string
	Brand       string
	HolderName  string
	ExpiryMonth int
	ExpiryYear  int
	IsDefault   bool
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Installment represents a payment installment for an event participant.
type Installment struct {
	ID            string
	EventID       string
	UserID        string
	ParticipantID string
	Amount        int64
	Status        string
	PaymentMethod string
	DueDate       time.Time
	PaidAt        *time.Time
}

// MetodoPagamento represents supported payment methods.
type MetodoPagamento string

const (
	MetodoPagamentoPix          MetodoPagamento = "PIX"
	MetodoPagamentoBoleto       MetodoPagamento = "BOLETO"
	MetodoPagamentoCartaoCredito MetodoPagamento = "CARTAO_CREDITO"
	MetodoPagamentoTransferencia MetodoPagamento = "TRANSFERENCIA"
)

// StatusPagamento represents payment lifecycle states.
type StatusPagamento string

const (
	StatusPagamentoPendente   StatusPagamento = "PENDENTE"
	StatusPagamentoPago       StatusPagamento = "PAGO"
	StatusPagamentoVencido    StatusPagamento = "VENCIDO"
	StatusPagamentoCancelado  StatusPagamento = "CANCELADO"
)

// PagamentoMensal is a recurring payment configuration for an event.
type PagamentoMensal struct {
	ID               string
	EventoID         string
	UsuarioID        string
	Descricao        string
	Valor            float64
	MetodoPagamento  MetodoPagamento
	Status           StatusPagamento
	DataVencimento   time.Time
	DataPagamento    *time.Time
	Observacao       string
	Comprovante      string // GridFS file ID
	CriadoEm        time.Time
	UpdatedAt        time.Time
}

// IsOwner returns true if userID is the payment owner.
func (p *PagamentoMensal) IsOwner(userID string) bool {
	return p.UsuarioID == userID
}

// IsPago returns true if the payment has been settled.
func (p *PagamentoMensal) IsPago() bool {
	return p.Status == StatusPagamentoPago
}

// CanPay returns true if the payment can be confirmed.
func (p *PagamentoMensal) CanPay() bool {
	return p.Status == StatusPagamentoPendente || p.Status == StatusPagamentoVencido
}

// EventoConfigPagamento holds payment settings for an event.
type EventoConfigPagamento struct {
	ID               string
	EventoID         string
	UsuarioID        string
	MetodosPagamento []MetodoPagamento
	PrazoPagamento   *time.Time
	ChavePix         string
	InstrucoesBoleto string
	CriadoEm        time.Time
	UpdatedAt        time.Time
}

// IsOwner returns true if userID is the config owner.
func (e *EventoConfigPagamento) IsOwner(userID string) bool {
	return e.UsuarioID == userID
}
