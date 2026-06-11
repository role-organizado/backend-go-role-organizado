// Package event — CSE_014 advanced endpoints.
// These use cases extend the basic event domain to mirror Java's EventoController
// (alterar-fase, upload-imagens, summaries, politica-convidados, detalhes,
// gerenciar, public-info, join, completo).
package event

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	paymentdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// =====================================================================
// AlterarFase — POST /api/eventos/v1/eventos/{id}/fase
// =====================================================================

// AlterarFase implements portin.AlterarFaseUseCase.
type AlterarFase struct {
	eventos       portout.EventoRepository
	participantes portout.ParticipanteRepository
	transactions  portout.PaymentTransactionRepository
}

// NewAlterarFase wires the AlterarFase use case.
func NewAlterarFase(
	e portout.EventoRepository,
	p portout.ParticipanteRepository,
	tx portout.PaymentTransactionRepository,
) *AlterarFase {
	return &AlterarFase{eventos: e, participantes: p, transactions: tx}
}

// Execute validates the state-machine transition, business rules, and updates fase.
func (uc *AlterarFase) Execute(ctx context.Context, in portin.AlterarFaseInput) (*portin.AlterarFaseResult, error) {
	if in.RequesterID == "" {
		return nil, apierr.BadRequest("header X-Usuario-Id é obrigatório")
	}
	if in.FaseDestino == "" {
		return nil, apierr.BadRequest("fase é obrigatória")
	}
	if !domain.IsValidFase(in.FaseDestino) {
		return nil, apierr.BadRequest(fmt.Sprintf("fase inválida: %s", in.FaseDestino))
	}

	evt, err := uc.eventos.FindByID(ctx, in.EventoID)
	if err != nil {
		return nil, err
	}

	authorized, err := uc.isOrganizador(ctx, evt, in.RequesterID)
	if err != nil {
		return nil, err
	}
	if !authorized {
		return nil, apierr.Forbidden("apenas ORGANIZADOR ou CO_ORGANIZADOR podem alterar a fase do evento")
	}

	currentFase := evt.Fase
	if currentFase == "" {
		currentFase = domain.FaseAguardandoAceite
	}
	destFase := domain.EventoFase(in.FaseDestino)

	if currentFase == destFase {
		return &portin.AlterarFaseResult{
			FaseAnterior: string(currentFase),
			FaseAtual:    string(destFase),
			Mensagem:     "fase já está em " + in.FaseDestino,
		}, nil
	}

	if !currentFase.CanTransitionTo(destFase) {
		return nil, apierr.BadRequest(fmt.Sprintf("transição de fase inválida: %s -> %s", currentFase, destFase))
	}

	// Rollback to AGUARDANDO_ACEITE blocked by any COMPLETED payment.
	if destFase == domain.FaseAguardandoAceite {
		txs, err := uc.transactions.FindByEventID(ctx, evt.ID)
		if err != nil {
			return nil, err
		}
		for _, t := range txs {
			if t.Status == paymentdomain.TransactionStatusCompleted {
				return nil, apierr.BadRequest("não é possível voltar para AGUARDANDO_ACEITE com pagamentos COMPLETED")
			}
		}
	}

	// Advance to PREPARACAO requires at least 1 non-organizador participant.
	if destFase == domain.FasePreparacao {
		count, err := uc.participantes.CountNonOrganizadorByEventID(ctx, evt.ID)
		if err != nil {
			return nil, err
		}
		if count < 1 {
			return nil, apierr.BadRequest("avanço para PREPARACAO requer ao menos um convidado")
		}
	}

	if err := uc.eventos.UpdateFase(ctx, evt.ID, destFase); err != nil {
		return nil, err
	}
	return &portin.AlterarFaseResult{
		FaseAnterior: string(currentFase),
		FaseAtual:    string(destFase),
		Mensagem:     "fase atualizada com sucesso",
	}, nil
}

func (uc *AlterarFase) isOrganizador(ctx context.Context, evt *domain.Evento, userID string) (bool, error) {
	if evt.IsOwner(userID) {
		return true, nil
	}
	return uc.participantes.HasOrganizadorPapel(ctx, evt.ID, userID)
}

// =====================================================================
// UploadImagens — POST /api/eventos/v1/eventos/{id}/imagens
// =====================================================================

const (
	maxImagensPorEvento = 10
	maxImagemSize       = 5 * 1024 * 1024 // 5MB
)

// ImagemStorage is the port the handler passes for image uploads. Returns
// (fileID, publicURL, error). The adapter wraps GridFSStorageAdapter.
type ImagemStorage interface {
	UploadImage(ctx context.Context, filename, contentType string, data []byte) (string, string, error)
}

// UploadImagens implements portin.UploadImagensUseCase.
type UploadImagens struct {
	eventos portout.EventoRepository
	storage ImagemStorage
}

// NewUploadImagens wires the UploadImagens use case.
func NewUploadImagens(e portout.EventoRepository, s ImagemStorage) *UploadImagens {
	return &UploadImagens{eventos: e, storage: s}
}

// Execute validates ownership/limits, uploads images, appends them to the event.
func (uc *UploadImagens) Execute(ctx context.Context, in portin.UploadImagensInput) (*domain.Evento, error) {
	if in.RequesterID == "" {
		return nil, apierr.BadRequest("header X-Usuario-Id é obrigatório")
	}
	if len(in.Imagens) == 0 {
		return nil, apierr.BadRequest("nenhuma imagem enviada")
	}
	evt, err := uc.eventos.FindByID(ctx, in.EventoID)
	if err != nil {
		return nil, err
	}
	if !evt.IsOwner(in.RequesterID) {
		return nil, apierr.Forbidden("apenas o organizador pode enviar imagens")
	}

	currentCount := len(evt.Imagens)
	if currentCount+len(in.Imagens) > maxImagensPorEvento {
		return nil, apierr.BadRequest(fmt.Sprintf("limite de %d imagens por evento excedido", maxImagensPorEvento))
	}

	tipo := in.Tipo
	if tipo == "" {
		tipo = "galeria"
	}

	now := time.Now().UTC()
	newImagens := make([]domain.EventoImagem, 0, len(in.Imagens))
	for i, img := range in.Imagens {
		if img.Size > maxImagemSize {
			return nil, apierr.BadRequest(fmt.Sprintf("imagem %s excede o tamanho máximo de 5MB", img.Filename))
		}
		if !isAllowedImageContentType(img.ContentType) {
			return nil, apierr.BadRequest(fmt.Sprintf("Content-Type não suportado: %s", img.ContentType))
		}
		_, url, err := uc.storage.UploadImage(ctx, img.Filename, img.ContentType, img.Data)
		if err != nil {
			return nil, apierr.Internal(err.Error())
		}
		ordem := currentCount + i
		t := tipo
		if ordem == 0 {
			t = "capa"
		}
		newImagens = append(newImagens, domain.EventoImagem{
			URL:          url,
			Ordem:        ordem,
			Tipo:         t,
			AdicionadaEm: now,
		})
	}

	if err := uc.eventos.AddImagens(ctx, evt.ID, newImagens); err != nil {
		return nil, err
	}
	evt.Imagens = append(evt.Imagens, newImagens...)
	evt.UpdatedAt = now
	return evt, nil
}

func isAllowedImageContentType(ct string) bool {
	switch ct {
	case "image/jpeg", "image/jpg", "image/png":
		return true
	}
	return false
}

// =====================================================================
// BuscarSummaries — POST /api/eventos/v1/eventos/summaries
// =====================================================================

const maxSummariesBatch = 50

// BuscarSummaries implements portin.BuscarSummariesUseCase.
type BuscarSummaries struct {
	eventos portout.EventoRepository
}

// NewBuscarSummaries wires the BuscarSummaries use case.
func NewBuscarSummaries(e portout.EventoRepository) *BuscarSummaries {
	return &BuscarSummaries{eventos: e}
}

// Execute batches a lookup by event IDs and projects EventoSummary values.
func (uc *BuscarSummaries) Execute(ctx context.Context, ids []string) ([]portin.EventoSummary, error) {
	if len(ids) == 0 {
		return nil, apierr.BadRequest("ids não pode ser vazio")
	}
	if len(ids) > maxSummariesBatch {
		return nil, apierr.BadRequest(fmt.Sprintf("máximo de %d ids por lote", maxSummariesBatch))
	}
	eventos, err := uc.eventos.FindAllByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]portin.EventoSummary, len(eventos))
	for i, e := range eventos {
		out[i] = portin.EventoSummary{
			ID:         e.ID,
			Nome:       e.Nome,
			Tipo:       e.Tipo,
			DataInicio: e.Data,
			DataFim:    e.DataFim,
			Local:      e.Local,
			Descricao:  e.Descricao,
			Status:     string(e.Status),
			ImageURL:   firstNonEmpty(e.ImageURL, firstCapaURL(e.Imagens)),
		}
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstCapaURL(imgs []domain.EventoImagem) string {
	for _, im := range imgs {
		if im.Tipo == "capa" {
			return im.URL
		}
	}
	if len(imgs) > 0 {
		return imgs[0].URL
	}
	return ""
}

// =====================================================================
// AtualizarPoliticaConvidados — PATCH /api/eventos/v1/eventos/{id}/politica-convidados
// =====================================================================

// AtualizarPoliticaConvidados implements portin.AtualizarPoliticaConvidadosUseCase.
type AtualizarPoliticaConvidados struct {
	eventos       portout.EventoRepository
	participantes portout.ParticipanteRepository
}

// NewAtualizarPoliticaConvidados wires the use case.
func NewAtualizarPoliticaConvidados(
	e portout.EventoRepository,
	p portout.ParticipanteRepository,
) *AtualizarPoliticaConvidados {
	return &AtualizarPoliticaConvidados{eventos: e, participantes: p}
}

// Execute validates the value and persists it.
func (uc *AtualizarPoliticaConvidados) Execute(ctx context.Context, in portin.AtualizarPoliticaConvidadosInput) (*domain.Evento, error) {
	if in.RequesterID == "" {
		return nil, apierr.BadRequest("header X-Usuario-Id é obrigatório")
	}
	if !domain.IsValidPoliticaConvidados(in.Politica) {
		return nil, apierr.BadRequest("politicaConvidados deve ser invite_only|public|approval")
	}
	evt, err := uc.eventos.FindByID(ctx, in.EventoID)
	if err != nil {
		return nil, err
	}
	authorized := evt.IsOwner(in.RequesterID)
	if !authorized {
		ok, err := uc.participantes.HasOrganizadorPapel(ctx, evt.ID, in.RequesterID)
		if err != nil {
			return nil, err
		}
		authorized = ok
	}
	if !authorized {
		return nil, apierr.Forbidden("apenas ORGANIZADOR ou CO_ORGANIZADOR podem atualizar a política de convidados")
	}

	if err := uc.eventos.UpdatePoliticaConvidados(ctx, evt.ID, in.Politica); err != nil {
		return nil, err
	}
	evt.PoliticaConvidados = in.Politica
	return evt, nil
}

// =====================================================================
// AtualizarDetalhes — PATCH /api/eventos/v1/eventos/{id}/detalhes
// =====================================================================

// AtualizarDetalhes implements portin.AtualizarDetalhesUseCase.
type AtualizarDetalhes struct {
	eventos       portout.EventoRepository
	participantes portout.ParticipanteRepository
}

// NewAtualizarDetalhes wires the use case.
func NewAtualizarDetalhes(
	e portout.EventoRepository,
	p portout.ParticipanteRepository,
) *AtualizarDetalhes {
	return &AtualizarDetalhes{eventos: e, participantes: p}
}

// Execute applies partial updates to nome/tipo/descricao/local/dates/endereco.
func (uc *AtualizarDetalhes) Execute(ctx context.Context, in portin.AtualizarDetalhesInput) (*domain.Evento, error) {
	if in.RequesterID == "" {
		return nil, apierr.BadRequest("header X-Usuario-Id é obrigatório")
	}
	if in.Nome != nil {
		n := *in.Nome
		if len(n) < 3 || len(n) > 100 {
			return nil, apierr.BadRequest("nome deve ter entre 3 e 100 caracteres")
		}
	}
	evt, err := uc.eventos.FindByID(ctx, in.EventoID)
	if err != nil {
		return nil, err
	}
	authorized := evt.IsOwner(in.RequesterID)
	if !authorized {
		ok, err := uc.participantes.HasOrganizadorPapel(ctx, evt.ID, in.RequesterID)
		if err != nil {
			return nil, err
		}
		authorized = ok
	}
	if !authorized {
		return nil, apierr.Forbidden("apenas ORGANIZADOR ou CO_ORGANIZADOR podem atualizar os detalhes")
	}

	if in.Nome != nil {
		evt.Nome = *in.Nome
	}
	if in.Tipo != nil {
		evt.Tipo = *in.Tipo
	}
	if in.Descricao != nil {
		evt.Descricao = *in.Descricao
	}
	if in.Local != nil {
		evt.Local = *in.Local
	}
	if in.DataInicio != nil {
		evt.Data = *in.DataInicio
	}
	if in.DataFim != nil {
		evt.DataFim = in.DataFim
	}
	if in.Endereco != nil {
		end := &domain.EventoEndereco{}
		if evt.Endereco != nil {
			end = evt.Endereco
		}
		if in.Endereco.Rua != nil {
			end.Rua = *in.Endereco.Rua
		}
		if in.Endereco.Numero != nil {
			end.Numero = *in.Endereco.Numero
		}
		if in.Endereco.Complemento != nil {
			end.Complemento = *in.Endereco.Complemento
		}
		if in.Endereco.Bairro != nil {
			end.Bairro = *in.Endereco.Bairro
		}
		if in.Endereco.Cidade != nil {
			end.Cidade = *in.Endereco.Cidade
		}
		if in.Endereco.Estado != nil {
			end.Estado = *in.Endereco.Estado
		}
		if in.Endereco.Cep != nil {
			end.Cep = *in.Endereco.Cep
		}
		if in.Endereco.PlaceID != nil {
			end.PlaceID = *in.Endereco.PlaceID
		}
		if in.Endereco.Latitude != nil {
			end.Latitude = in.Endereco.Latitude
		}
		if in.Endereco.Longitude != nil {
			end.Longitude = in.Endereco.Longitude
		}
		evt.Endereco = end
	}

	return uc.eventos.UpdateDetalhes(ctx, evt)
}

// =====================================================================
// GerenciarEvento — GET /api/eventos/v1/eventos/{id}/gerenciar
// =====================================================================

// GerenciarEvento implements portin.GerenciarEventoUseCase.
type GerenciarEvento struct {
	eventos       portout.EventoRepository
	participantes portout.ParticipanteRepository
	transactions  portout.PaymentTransactionRepository
}

// NewGerenciarEvento wires the use case.
func NewGerenciarEvento(
	e portout.EventoRepository,
	p portout.ParticipanteRepository,
	tx portout.PaymentTransactionRepository,
) *GerenciarEvento {
	return &GerenciarEvento{eventos: e, participantes: p, transactions: tx}
}

// Execute aggregates the organizer dashboard view.
func (uc *GerenciarEvento) Execute(ctx context.Context, in portin.GerenciarEventoInput) (*portin.GerenciarEventoResult, error) {
	if in.RequesterID == "" {
		return nil, apierr.BadRequest("header X-Usuario-Id é obrigatório")
	}
	evt, err := uc.eventos.FindByID(ctx, in.EventoID)
	if err != nil {
		return nil, err
	}
	authorized := evt.IsOwner(in.RequesterID)
	if !authorized {
		ok, err := uc.participantes.HasOrganizadorPapel(ctx, evt.ID, in.RequesterID)
		if err != nil {
			return nil, err
		}
		authorized = ok
	}
	if !authorized {
		return nil, apierr.Forbidden("apenas ORGANIZADOR ou CO_ORGANIZADOR podem acessar o dashboard")
	}

	confirmed, err := uc.participantes.CountByEventIDAndStatus(ctx, evt.ID, "CONFIRMADO")
	if err != nil {
		return nil, err
	}
	pending, err := uc.participantes.CountByEventIDAndStatus(ctx, evt.ID, "PENDENTE")
	if err != nil {
		return nil, err
	}
	declined, err := uc.participantes.CountByEventIDAndStatus(ctx, evt.ID, "RECUSADO")
	if err != nil {
		return nil, err
	}

	txs, err := uc.transactions.FindByEventID(ctx, evt.ID)
	if err != nil {
		return nil, err
	}
	hasCompleted := false
	for _, t := range txs {
		if t.Status == paymentdomain.TransactionStatusCompleted {
			hasCompleted = true
			break
		}
	}

	fase := string(evt.Fase)
	if fase == "" {
		fase = string(domain.FaseAguardandoAceite)
	}

	return &portin.GerenciarEventoResult{
		EventoID:              evt.ID,
		NomeEvento:            evt.Nome,
		Fase:                  fase,
		PaymentReleaseTrigger: evt.PaymentReleaseTrigger,
		RateiosHabilitado:     evt.RateiosHabilitado,
		PagamentosHabilitado:  evt.PagamentosHabilitado,
		ParticipantSummary: portin.ParticipantSummary{
			Total:     int(confirmed + pending + declined),
			Confirmed: int(confirmed),
			Pending:   int(pending),
			Declined:  int(declined),
		},
		HasCompletedPayments: hasCompleted,
		WorkflowStatus:       "NOT_STARTED",
	}, nil
}

// =====================================================================
// GetPublicInfo — GET /api/v1/eventos/{eventId}/public-info  (NO AUTH)
// =====================================================================

// GetPublicInfo implements portin.GetPublicInfoUseCase.
type GetPublicInfo struct {
	eventos       portout.EventoRepository
	participantes portout.ParticipanteRepository
	usuarios      portout.UsuarioRepository
}

// NewGetPublicInfo wires the use case.
func NewGetPublicInfo(
	e portout.EventoRepository,
	p portout.ParticipanteRepository,
	u portout.UsuarioRepository,
) *GetPublicInfo {
	return &GetPublicInfo{eventos: e, participantes: p, usuarios: u}
}

// Execute returns the public projection for the given event.
func (uc *GetPublicInfo) Execute(ctx context.Context, eventoID string) (*portin.EventoPublicInfoResult, error) {
	evt, err := uc.eventos.FindByID(ctx, eventoID)
	if err != nil {
		return nil, err
	}
	politica := evt.PoliticaConvidados
	if politica == "" {
		politica = domain.PoliticaInviteOnly
	}
	var organizadorNome *string
	if uc.usuarios != nil && evt.UsuarioID != "" {
		if u, err := uc.usuarios.FindByID(ctx, evt.UsuarioID); err == nil && u != nil {
			n := u.Nome
			organizadorNome = &n
		}
	}
	totalConfirmados, err := uc.participantes.CountConfirmedByEventID(ctx, evt.ID)
	if err != nil {
		return nil, err
	}
	imagem := firstNonEmpty(evt.ImageURL, firstCapaURL(evt.Imagens))
	return &portin.EventoPublicInfoResult{
		EventID:            evt.ID,
		Nome:               evt.Nome,
		Tipo:               evt.Tipo,
		Descricao:          evt.Descricao,
		Local:              evt.Local,
		DataInicio:         evt.Data,
		DataFim:            evt.DataFim,
		OrganizadorNome:    organizadorNome,
		PoliticaConvidados: politica,
		LimiteConvidados:   evt.LimiteConvidados,
		TotalConfirmados:   totalConfirmados,
		ImagemCapa:         imagem,
	}, nil
}

// =====================================================================
// JoinEvento — POST /api/v1/eventos/{eventId}/join
// =====================================================================

// JoinEvento implements portin.JoinEventoUseCase.
type JoinEvento struct {
	eventos       portout.EventoRepository
	participantes portout.ParticipanteRepository
}

// NewJoinEvento wires the use case.
func NewJoinEvento(
	e portout.EventoRepository,
	p portout.ParticipanteRepository,
) *JoinEvento {
	return &JoinEvento{eventos: e, participantes: p}
}

// Execute processes a public link join request.
func (uc *JoinEvento) Execute(ctx context.Context, in portin.JoinEventoInput) (*portin.JoinEventoResult, error) {
	if in.UserID == "" {
		return nil, apierr.Unauthorized("autenticação necessária")
	}
	evt, err := uc.eventos.FindByID(ctx, in.EventoID)
	if err != nil {
		return nil, err
	}
	politica := evt.PoliticaConvidados
	if politica == "" {
		politica = domain.PoliticaInviteOnly
	}

	if politica == domain.PoliticaInviteOnly {
		return nil, apierr.Forbidden("este evento só permite convites diretos do organizador")
	}

	exists, err := uc.participantes.IsParticipantOfEvent(ctx, evt.ID, in.UserID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apierr.Conflict("usuário já é participante deste evento")
	}

	// Capacity check.
	if evt.LimiteConvidados != nil {
		confirmed, err := uc.participantes.CountConfirmedByEventID(ctx, evt.ID)
		if err != nil {
			return nil, err
		}
		if int(confirmed) >= *evt.LimiteConvidados {
			return nil, apierr.Unprocessable("evento atingiu o limite de convidados")
		}
	}

	status := "PENDENTE"
	httpStatus := http.StatusAccepted // 202 for approval
	if politica == domain.PoliticaPublic {
		status = "CONFIRMADO"
		httpStatus = http.StatusOK
	}

	id, err := uc.participantes.CreateParticipant(ctx, portout.NewParticipant{
		EventID:          evt.ID,
		UserID:           in.UserID,
		TipoParticipante: "USER",
		Papel:            "CONVIDADO",
		Status:           status,
	})
	if err != nil {
		return nil, err
	}
	return &portin.JoinEventoResult{
		Status:         status,
		ParticipantID:  id,
		HTTPStatusCode: httpStatus,
	}, nil
}

// =====================================================================
// GetEventoCompleto — GET /api/eventos/v1/eventos/{id}/completo
// =====================================================================

// GetEventoCompleto implements portin.GetEventoCompletoUseCase.
type GetEventoCompleto struct {
	eventos       portout.EventoRepository
	participants  portout.ParticipantRepository
	rateios       portout.RateioRepository
}

// NewGetEventoCompleto wires the use case.
func NewGetEventoCompleto(
	e portout.EventoRepository,
	p portout.ParticipantRepository,
	r portout.RateioRepository,
) *GetEventoCompleto {
	return &GetEventoCompleto{eventos: e, participants: p, rateios: r}
}

// Execute aggregates the evento + participants + rateios projection.
func (uc *GetEventoCompleto) Execute(ctx context.Context, eventoID string) (*portin.EventoCompletoResult, error) {
	evt, err := uc.eventos.FindByID(ctx, eventoID)
	if err != nil {
		return nil, err
	}

	parts, err := uc.participants.FindAllByEventID(ctx, evt.ID)
	if err != nil {
		return nil, err
	}
	pInfo := make([]portin.EventoParticipantInfo, len(parts))
	for i, p := range parts {
		pInfo[i] = portin.EventoParticipantInfo{
			ID:     p.ID,
			UserID: p.UserID,
			Name:   p.Name,
			Email:  p.Email,
			Status: p.Status,
		}
	}

	rats, err := uc.rateios.FindByEventoID(ctx, evt.ID)
	if err != nil {
		return nil, err
	}
	rInfo := make([]portin.EventoRateioInfo, len(rats))
	for i, r := range rats {
		items := make([]portin.EventoRateioItemInfo, len(r.Itens))
		for j, it := range r.Itens {
			items[j] = portin.EventoRateioItemInfo{
				ID:         it.ID,
				Descricao:  it.Descricao,
				Valor:      it.Valor,
				Quantidade: it.Quantidade,
				Total:      it.Total,
			}
		}
		rInfo[i] = portin.EventoRateioInfo{
			ID:         r.ID,
			Descricao:  r.Descricao,
			Tipo:       string(r.Tipo),
			Status:     string(r.Status),
			ValorTotal: r.ValorTotal,
			Itens:      items,
		}
	}

	imgs := append([]domain.EventoImagem(nil), evt.Imagens...)
	sort.SliceStable(imgs, func(i, j int) bool { return imgs[i].Ordem < imgs[j].Ordem })

	return &portin.EventoCompletoResult{
		ID:                   evt.ID,
		Nome:                 evt.Nome,
		Descricao:            evt.Descricao,
		Local:                evt.Local,
		DataInicio:           evt.Data,
		DataFim:              evt.DataFim,
		Endereco:             evt.Endereco,
		UsuarioIDResponsavel: evt.UsuarioID,
		Tipo:                 evt.Tipo,
		Status:               string(evt.Status),
		CriadoEm:             evt.CriadoEm,
		AtualizadoEm:         evt.UpdatedAt,
		Imagens:              imgs,
		Usuarios:             pInfo,
		ConvidadosExternos:   []any{},
		Rateios:              rInfo,
	}, nil
}
