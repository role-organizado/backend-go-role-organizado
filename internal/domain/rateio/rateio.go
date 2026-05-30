package rateio

import "time"

// TipoRateio define como os custos são distribuídos.
type TipoRateio string

const (
	TipoRateioPercentual TipoRateio = "PERCENTUAL"
	TipoRateioDivisao    TipoRateio = "DIVISAO"
	TipoRateioItens      TipoRateio = "ITENS"
)

// StatusRateio representa o estado de um rateio.
type StatusRateio string

const (
	StatusRateioAberto   StatusRateio = "ABERTO"
	StatusRateioFechado  StatusRateio = "FECHADO"
)

// RateioItem is a line-item in a ITENS-type rateio.
type RateioItem struct {
	ID         string
	RateioID   string
	Descricao  string
	Valor      float64
	Quantidade int
	Total      float64
	CriadoEm  time.Time
	UpdatedAt  time.Time
}

// Rateio represents a cost split for an event.
type Rateio struct {
	ID               string
	EventoID         string
	UsuarioID        string // owner
	Tipo             TipoRateio
	Status           StatusRateio
	Descricao        string
	ValorTotal       float64
	NumeroParticipantes int
	Itens            []RateioItem
	CriadoEm         time.Time
	UpdatedAt        time.Time
}

// IsOwner checks if the given user owns this rateio.
func (r *Rateio) IsOwner(userID string) bool {
	return r.UsuarioID == userID
}

// IsFechado checks if the rateio has been closed.
func (r *Rateio) IsFechado() bool {
	return r.Status == StatusRateioFechado
}

// CanEdit returns true when the rateio can still be modified.
func (r *Rateio) CanEdit() bool {
	return r.Status == StatusRateioAberto
}

// PreviewParticipante holds the calculated amount for a single participant.
type PreviewParticipante struct {
	UsuarioID string
	Valor     float64
	Percentual float64
}

// PreviewResult is the output of the rateio preview calculation.
type PreviewResult struct {
	RateioID      string
	TotalGeral    float64
	Participantes []PreviewParticipante
}
