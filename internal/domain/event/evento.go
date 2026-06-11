package event

import "time"

// Evento represents a finalized event.
type Evento struct {
	ID                   string
	UsuarioID            string
	Nome                 string
	Tipo                 string
	Data                 time.Time
	DataFim              *time.Time
	Descricao            string
	Local                string
	FotoURL              string
	Status               EventoStatus
	ConvidadosIDs        []string
	PoliticaConvidados   string
	LimiteConvidados     *int
	RateiosHabilitado    bool
	TipoDivisaoRateio    string
	PagamentosHabilitado bool
	MetodosPagamento     []string
	PrazoPagamento       *time.Time
	RegrasCustomizadas   string
	PoliticaCancelamento string
	ModulosAtivos        []string       // active niche modules (e.g. COFRINHO, LISTA_COLABORATIVA)
	ConfiguracaoNicho    map[string]any // niche-specific configuration

	// Advanced fields (CSE_014 — eventos-advanced).
	Fase                  EventoFase
	PaymentReleaseTrigger string
	Endereco              *EventoEndereco
	ImageURL              string // deprecated, kept for parity with Java
	Imagens               []EventoImagem
	CriadoEm              time.Time
	UpdatedAt             time.Time
}

// EventoStatus represents the lifecycle of an event.
type EventoStatus string

const (
	EventoStatusRascunho  EventoStatus = "RASCUNHO"
	EventoStatusPublicado EventoStatus = "PUBLICADO"
	EventoStatusCancelado EventoStatus = "CANCELADO"
	EventoStatusConcluido EventoStatus = "CONCLUIDO"
)

// EventoFase represents the operational phase of a published event.
// Mirrors Java's EventoFase enum.
type EventoFase string

const (
	FaseOrganizacao       EventoFase = "ORGANIZACAO"
	FaseAguardandoAceite  EventoFase = "AGUARDANDO_ACEITE"
	FaseColetaPagamentos  EventoFase = "COLETA_PAGAMENTOS"
	FasePreparacao        EventoFase = "PREPARACAO"
	FaseExecucao          EventoFase = "EXECUCAO"
	FaseFinalizado        EventoFase = "FINALIZADO"
)

// PoliticaConvidados allowed values (Java parity).
const (
	PoliticaInviteOnly = "invite_only"
	PoliticaPublic     = "public"
	PoliticaApproval   = "approval"
)

// EventoImagem represents an image attached to an event.
type EventoImagem struct {
	URL          string
	Ordem        int
	Tipo         string // capa | galeria
	AdicionadaEm time.Time
}

// EventoEndereco is the structured address of the event.
type EventoEndereco struct {
	Rua         string
	Numero      string
	Complemento string
	Bairro      string
	Cidade      string
	Estado      string
	Cep         string
	PlaceID     string
	Latitude    *float64
	Longitude   *float64
}

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

// IsValidPoliticaConvidados reports whether v is one of the allowed values.
func IsValidPoliticaConvidados(v string) bool {
	return v == PoliticaInviteOnly || v == PoliticaPublic || v == PoliticaApproval
}

// CanTransitionTo validates a fase state-machine transition.
// Mirrors Java EventoController.alterarFase allowed transitions.
func (f EventoFase) CanTransitionTo(dest EventoFase) bool {
	switch f {
	case FaseAguardandoAceite:
		return dest == FaseColetaPagamentos || dest == FasePreparacao
	case FaseColetaPagamentos:
		return dest == FaseAguardandoAceite
	case FasePreparacao:
		return dest == FaseExecucao
	}
	return false
}

// IsValidFase returns true if the string maps to a known EventoFase.
func IsValidFase(s string) bool {
	switch EventoFase(s) {
	case FaseOrganizacao, FaseAguardandoAceite, FaseColetaPagamentos,
		FasePreparacao, FaseExecucao, FaseFinalizado:
		return true
	}
	return false
}
