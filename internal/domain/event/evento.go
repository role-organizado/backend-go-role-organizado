package event

import "time"

// Evento represents a finalized event.
type Evento struct {
	ID                  string
	UsuarioID           string
	Nome                string
	Tipo                string
	Data                time.Time
	Descricao           string
	Local               string
	FotoURL             string
	Status              EventoStatus
	ConvidadosIDs       []string
	PoliticaConvidados  string
	LimiteConvidados    *int
	RateiosHabilitado   bool
	TipoDivisaoRateio   string
	PagamentosHabilitado bool
	MetodosPagamento    []string
	PrazoPagamento      *time.Time
	RegrasCustomizadas  string
	PoliticaCancelamento string
	ModulosAtivos       []string       // active niche modules (e.g. COFRINHO, LISTA_COLABORATIVA)
	ConfiguracaoNicho   map[string]any // niche-specific configuration
	CriadoEm           time.Time
	UpdatedAt           time.Time
}

// EventoStatus represents the lifecycle of an event.
type EventoStatus string

const (
	EventoStatusRascunho   EventoStatus = "RASCUNHO"
	EventoStatusPublicado  EventoStatus = "PUBLICADO"
	EventoStatusCancelado  EventoStatus = "CANCELADO"
	EventoStatusConcluido  EventoStatus = "CONCLUIDO"
)

// Convidado represents a guest added to a published event.
type Convidado struct {
	Telefone string
	Nome     string
}

// IsOwner returns true if the given userID owns this event.
func (e *Evento) IsOwner(userID string) bool {
	return e.UsuarioID == userID
}

// CanEdit returns true if the event is in a state that allows editing.
func (e *Evento) CanEdit() bool {
	return e.Status == EventoStatusRascunho || e.Status == EventoStatusPublicado
}
