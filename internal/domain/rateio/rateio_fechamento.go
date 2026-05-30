package rateio

import "time"

// RateioFechamento represents a versioned closing/snapshot of a rateio.
type RateioFechamento struct {
	ID        string
	RateioID  string
	EventoID  string
	Versao    int
	ValorTotal float64
	Participantes []FechamentoParticipante
	CriadoEm  time.Time
}

// FechamentoParticipante is the calculated share for one participant at closing time.
type FechamentoParticipante struct {
	UsuarioID  string
	Valor      float64
	Percentual float64
	Pago       bool
	PagoEm    *time.Time
}
