package event

import "time"

// EventoDraft represents a multi-step event creation wizard draft (5 etapas).
// TTL: 90 days from last update in MongoDB.
type EventoDraft struct {
	ID        string
	UsuarioID string

	// Etapa 0 — basic info
	Nome      string
	Tipo      string
	Data      *time.Time
	Descricao string
	Local     string

	// Etapa 1 — guests
	ConvidadosIDs      []string
	PoliticaConvidados string
	LimiteConvidados   *int

	// Etapa 2 — cost split
	RateiosHabilitado  bool
	RateiosItens       []RateioItem
	TipoDivisaoRateio  string

	// Etapa 3 — payment
	PagamentosHabilitado bool
	MetodosPagamento     []string
	PrazoPagamento       *time.Time

	// Etapa 4 — rules
	RegrasCustomizadas   string
	PoliticaCancelamento string

	// Wizard state
	EtapaAtual      int
	EtapasCompletas []int

	CriadoEm  time.Time
	UpdatedAt time.Time
}

// RateioItem represents a cost split item in a draft.
type RateioItem struct {
	Descricao  string
	Valor      float64
	Quantidade int
}

// IsOwner returns true if the given userID owns this draft.
func (d *EventoDraft) IsOwner(userID string) bool {
	return d.UsuarioID == userID
}

// EtapaCompleta marks the given etapa as complete.
func (d *EventoDraft) EtapaCompleta(etapa int) {
	for _, e := range d.EtapasCompletas {
		if e == etapa {
			return
		}
	}
	d.EtapasCompletas = append(d.EtapasCompletas, etapa)
}

// IsEtapaCompleta checks if the given etapa has been completed.
func (d *EventoDraft) IsEtapaCompleta(etapa int) bool {
	for _, e := range d.EtapasCompletas {
		if e == etapa {
			return true
		}
	}
	return false
}

// HasConflict returns true if the draft has been modified since lastReadAt
// (used for optimistic concurrency detection by the frontend).
func (d *EventoDraft) HasConflict(lastReadAt time.Time) bool {
	return d.UpdatedAt.After(lastReadAt)
}
