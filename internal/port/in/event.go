package in

import (
	"context"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// ---- Evento inputs ----

// CreateEventoInput represents the data needed to create a published event.
type CreateEventoInput struct {
	UsuarioID            string
	Nome                 string
	Tipo                 string
	Data                 time.Time
	Descricao            string
	Local                string
	FotoURL              string
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
}

// UpdateEventoInput holds fields that can be updated on an existing event.
type UpdateEventoInput struct {
	Nome                 string
	Tipo                 string
	Data                 *time.Time
	Descricao            string
	Local                string
	FotoURL              string
	PoliticaConvidados   string
	LimiteConvidados     *int
	RateiosHabilitado    *bool
	TipoDivisaoRateio    string
	PagamentosHabilitado *bool
	MetodosPagamento     []string
	PrazoPagamento       *time.Time
	RegrasCustomizadas   string
	PoliticaCancelamento string
}

// CreateEventoUseCase creates a new event.
type CreateEventoUseCase interface {
	Execute(ctx context.Context, in CreateEventoInput) (*event.Evento, error)
}

// GetEventoUseCase retrieves an event by ID.
type GetEventoUseCase interface {
	Execute(ctx context.Context, id string) (*event.Evento, error)
}

// ListEventosUseCase lists events, optionally filtered by owner.
type ListEventosUseCase interface {
	Execute(ctx context.Context, usuarioID *string, page, pageSize int) ([]event.Evento, int64, error)
}

// UpdateEventoUseCase updates an existing event.
type UpdateEventoUseCase interface {
	Execute(ctx context.Context, id, requesterID string, in UpdateEventoInput) (*event.Evento, error)
}

// DeleteEventoUseCase deletes an event by ID.
type DeleteEventoUseCase interface {
	Execute(ctx context.Context, id, requesterID string) error
}

// ListEventosByUsuarioInput holds parameters for the user-filtered cursor-paginated listing.
type ListEventosByUsuarioInput struct {
	UsuarioID     string
	RequesterID   string
	Status        *string
	Tipo          *string
	DataInicioGte *time.Time
	DataInicioLte *time.Time
	Cursor        *string
	Limit         int
}

// ListEventosByUsuarioUseCase lists events belonging to a specific user (cursor pagination).
type ListEventosByUsuarioUseCase interface {
	Execute(ctx context.Context, in ListEventosByUsuarioInput) (portout.EventosCursorPage, error)
}

// ---- EventoDraft inputs ----

// DraftRateioItem is used in draft upsert inputs.
type DraftRateioItem struct {
	Descricao  string
	Valor      float64
	Quantidade int
}

// UpsertDraftInput represents a draft auto-save payload.
// All fields are optional — only non-nil/non-zero fields are applied.
type UpsertDraftInput struct {
	// Etapa 0
	Nome      *string
	Tipo      *string
	Data      *time.Time
	Descricao *string
	Local     *string

	// Etapa 1
	ConvidadosIDs      []string
	PoliticaConvidados *string
	LimiteConvidados   *int

	// Etapa 2
	RateiosHabilitado *bool
	RateiosItens      []DraftRateioItem
	TipoDivisaoRateio *string

	// Etapa 3
	PagamentosHabilitado *bool
	MetodosPagamento     []string
	PrazoPagamento       *time.Time

	// Etapa 4
	RegrasCustomizadas   *string
	PoliticaCancelamento *string

	// Wizard state
	EtapaAtual      *int
	EtapasCompletas []int

	// Optimistic concurrency: client last-read timestamp
	LastReadAt *time.Time
}

// CreateDraftUseCase creates a new empty draft.
type CreateDraftUseCase interface {
	Execute(ctx context.Context, usuarioID string) (*event.EventoDraft, error)
}

// GetDraftUseCase retrieves a draft by ID.
type GetDraftUseCase interface {
	Execute(ctx context.Context, id, requesterID string) (*event.EventoDraft, error)
}

// ListDraftsUseCase lists all drafts for a user.
type ListDraftsUseCase interface {
	Execute(ctx context.Context, usuarioID string) ([]event.EventoDraft, error)
}

// UpdateDraftUseCase applies partial updates to a draft (auto-save).
type UpdateDraftUseCase interface {
	Execute(ctx context.Context, id, requesterID string, in UpsertDraftInput) (*event.EventoDraft, error)
}

// DeleteDraftUseCase deletes a draft by ID.
type DeleteDraftUseCase interface {
	Execute(ctx context.Context, id, requesterID string) error
}

// PublishDraftUseCase converts a completed draft into a published event.
type PublishDraftUseCase interface {
	Execute(ctx context.Context, draftID, requesterID string) (*event.Evento, error)
}

// ValidationResult holds the outcome of a single draft field validation.
type ValidationResult struct {
	Field   string `json:"field"`
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

// ValidateDraftUseCase validates that a draft has all required fields filled in.
// Returns a slice of ValidationResult — one per checked field.
type ValidateDraftUseCase interface {
	Execute(ctx context.Context, id, requesterID string) ([]ValidationResult, error)
}

// AddConvidadosInput holds the parameters for adding guests to a published event.
type AddConvidadosInput struct {
	EventoID   string
	UsuarioID  string
	Convidados []event.Convidado
}

// AddConvidadosUseCase adds convidados (guests) to a published event.
type AddConvidadosUseCase interface {
	Execute(ctx context.Context, in AddConvidadosInput) error
}

// ---- Eventos advanced (CSE_014) inputs/use-cases ----

// AlterarFaseInput holds the parameters for the fase transition use case.
type AlterarFaseInput struct {
	EventoID    string
	RequesterID string
	FaseDestino string
}

// AlterarFaseResult is the response payload for AlterarFaseUseCase.
type AlterarFaseResult struct {
	FaseAnterior string
	FaseAtual    string
	Mensagem     string
}

// AlterarFaseUseCase advances/rolls back the operational fase of an event.
type AlterarFaseUseCase interface {
	Execute(ctx context.Context, in AlterarFaseInput) (*AlterarFaseResult, error)
}

// UploadImagemInput represents a single image to be uploaded.
type UploadImagemInput struct {
	Filename    string
	ContentType string
	Size        int64
	Data        []byte
}

// UploadImagensInput holds parameters for the upload-imagens use case.
type UploadImagensInput struct {
	EventoID    string
	RequesterID string
	Tipo        string // default 'galeria'; first image stored as 'capa'
	Imagens     []UploadImagemInput
}

// UploadImagensUseCase uploads images to GridFS and appends them to the event.
type UploadImagensUseCase interface {
	Execute(ctx context.Context, in UploadImagensInput) (*event.Evento, error)
}

// EventoSummary is the lightweight projection returned by BuscarSummaries.
type EventoSummary struct {
	ID         string     `json:"id"`
	Nome       string     `json:"nome"`
	Tipo       string     `json:"tipo"`
	DataInicio time.Time  `json:"dataInicio"`
	DataFim    *time.Time `json:"dataFim,omitempty"`
	Local      string     `json:"local"`
	Descricao  string     `json:"descricao"`
	Status     string     `json:"status"`
	ImageURL   string     `json:"imageUrl,omitempty"`
}

// BuscarSummariesUseCase fetches lightweight summaries for a batch of event IDs.
type BuscarSummariesUseCase interface {
	Execute(ctx context.Context, ids []string) ([]EventoSummary, error)
}

// AtualizarPoliticaConvidadosInput holds parameters for updating politica.
type AtualizarPoliticaConvidadosInput struct {
	EventoID    string
	RequesterID string
	Politica    string
}

// AtualizarPoliticaConvidadosUseCase updates the event's politica de convidados.
type AtualizarPoliticaConvidadosUseCase interface {
	Execute(ctx context.Context, in AtualizarPoliticaConvidadosInput) (*event.Evento, error)
}

// EnderecoInput is the request-side address payload.
type EnderecoInput struct {
	Rua         *string
	Numero      *string
	Complemento *string
	Bairro      *string
	Cidade      *string
	Estado      *string
	Cep         *string
	PlaceID     *string
	Latitude    *float64
	Longitude   *float64
}

// AtualizarDetalhesInput holds parameters for the partial update of evento details.
type AtualizarDetalhesInput struct {
	EventoID    string
	RequesterID string
	Nome        *string
	Tipo        *string
	Descricao   *string
	Local       *string
	DataInicio  *time.Time
	DataFim     *time.Time
	Endereco    *EnderecoInput
}

// AtualizarDetalhesUseCase updates editable detail fields of a published event.
type AtualizarDetalhesUseCase interface {
	Execute(ctx context.Context, in AtualizarDetalhesInput) (*event.Evento, error)
}

// ParticipantSummary aggregates participant counts for an event.
type ParticipantSummary struct {
	Total      int `json:"total"`
	Confirmed  int `json:"confirmed"`
	Pending    int `json:"pending"`
	Declined   int `json:"declined"`
}

// GerenciarEventoResult is the dashboard view returned by GerenciarEventoUseCase.
type GerenciarEventoResult struct {
	EventoID              string             `json:"eventoId"`
	NomeEvento            string             `json:"nomeEvento"`
	Fase                  string             `json:"fase"`
	PaymentReleaseTrigger string             `json:"paymentReleaseTrigger"`
	RateiosHabilitado     bool               `json:"rateiosHabilitado"`
	PagamentosHabilitado  bool               `json:"pagamentosHabilitado"`
	ParticipantSummary    ParticipantSummary `json:"participantSummary"`
	HasCompletedPayments  bool               `json:"hasCompletedPayments"`
	WorkflowStatus        string             `json:"workflowStatus"`
}

// GerenciarEventoInput holds parameters for the dashboard query.
type GerenciarEventoInput struct {
	EventoID    string
	RequesterID string
}

// GerenciarEventoUseCase returns the dashboard view for organizers.
type GerenciarEventoUseCase interface {
	Execute(ctx context.Context, in GerenciarEventoInput) (*GerenciarEventoResult, error)
}

// EventoPublicInfoResult is the no-auth public projection of an event.
type EventoPublicInfoResult struct {
	EventID            string     `json:"eventId"`
	Nome               string     `json:"nome"`
	Tipo               string     `json:"tipo"`
	Descricao          string     `json:"descricao"`
	Local              string     `json:"local"`
	DataInicio         time.Time  `json:"dataInicio"`
	DataFim            *time.Time `json:"dataFim,omitempty"`
	OrganizadorNome    *string    `json:"organizadorNome,omitempty"`
	PoliticaConvidados string     `json:"politicaConvidados"`
	LimiteConvidados   *int       `json:"limiteConvidados,omitempty"`
	TotalConfirmados   int64      `json:"totalConfirmados"`
	ImagemCapa         string     `json:"imagemCapa,omitempty"`
}

// GetPublicInfoUseCase returns the public-info view for an event.
type GetPublicInfoUseCase interface {
	Execute(ctx context.Context, eventoID string) (*EventoPublicInfoResult, error)
}

// JoinEventoInput holds parameters for the join use case.
type JoinEventoInput struct {
	EventoID string
	UserID   string
}

// JoinEventoResult tells the handler which HTTP status code to apply.
type JoinEventoResult struct {
	Status         string // CONFIRMADO | PENDENTE
	ParticipantID  string
	HTTPStatusCode int // 200 for public/CONFIRMADO; 202 for approval/PENDENTE
}

// JoinEventoUseCase processes a public join via shared link.
type JoinEventoUseCase interface {
	Execute(ctx context.Context, in JoinEventoInput) (*JoinEventoResult, error)
}

// EventoCompletoResult aggregates evento + participants + rateios + image list.
type EventoCompletoResult struct {
	ID                   string                  `json:"id"`
	Nome                 string                  `json:"nome"`
	Descricao            string                  `json:"descricao"`
	Local                string                  `json:"local"`
	DataInicio           time.Time               `json:"dataInicio"`
	DataFim              *time.Time              `json:"dataFim,omitempty"`
	Endereco             *event.EventoEndereco   `json:"endereco,omitempty"`
	UsuarioIDResponsavel string                  `json:"usuarioIdResponsavel"`
	Tipo                 string                  `json:"tipo"`
	Status               string                  `json:"status"`
	CriadoEm             time.Time               `json:"criadoEm"`
	AtualizadoEm         time.Time               `json:"atualizadoEm"`
	Imagens              []event.EventoImagem    `json:"imagens"`
	Usuarios             []EventoParticipantInfo `json:"usuarios"`
	ConvidadosExternos   []any                   `json:"convidadosExternos"`
	Rateios              []EventoRateioInfo      `json:"rateios"`
}

// EventoParticipantInfo is a lightweight participant projection for completo.
type EventoParticipantInfo struct {
	ID      string `json:"id"`
	UserID  string `json:"userId"`
	Name    string `json:"name,omitempty"`
	Email   string `json:"email,omitempty"`
	Status  string `json:"status"`
}

// EventoRateioInfo is a lightweight rateio projection for completo.
type EventoRateioInfo struct {
	ID         string                  `json:"id"`
	Descricao  string                  `json:"descricao"`
	Tipo       string                  `json:"tipo"`
	Status     string                  `json:"status"`
	ValorTotal float64                 `json:"valorTotal"`
	Itens      []EventoRateioItemInfo  `json:"itens"`
}

// EventoRateioItemInfo is a single rateio item projection.
type EventoRateioItemInfo struct {
	ID         string  `json:"id"`
	Descricao  string  `json:"descricao"`
	Valor      float64 `json:"valor"`
	Quantidade int     `json:"quantidade"`
	Total      float64 `json:"total"`
}

// GetEventoCompletoUseCase returns the aggregated detail view.
type GetEventoCompletoUseCase interface {
	Execute(ctx context.Context, eventoID string) (*EventoCompletoResult, error)
}

// ---- Event publication monitoring ----

// FindStuckExecutionsInput parameterizes the stuck-execution scan run by the
// EventPublicationMonitoringWorkflow.
type FindStuckExecutionsInput struct {
	// StuckThresholdMinutes is how old (in minutes) a PENDING/PROCESSING execution
	// must be before it is considered stuck.
	StuckThresholdMinutes int
	// MaxResults caps the number of stuck executions returned in a single scan.
	MaxResults int
}

// StuckExecution describes a single execution flagged as stuck.
type StuckExecution struct {
	TransactionID string    `json:"transactionId"`
	EventID       string    `json:"eventId"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	AgeMinutes    int       `json:"ageMinutes"`
}

// FindStuckExecutionsResult is the outcome of a stuck-execution scan.
type FindStuckExecutionsResult struct {
	StuckCount int              `json:"stuckCount"`
	Executions []StuckExecution `json:"executions"`
}

// FindStuckExecutionsUseCase scans for payment executions that have been pending
// longer than a threshold and reports them so the monitoring workflow can raise
// alerts. Read-only and idempotent — safe to run on a fixed cadence.
type FindStuckExecutionsUseCase interface {
	Execute(ctx context.Context, in FindStuckExecutionsInput) (*FindStuckExecutionsResult, error)
}
