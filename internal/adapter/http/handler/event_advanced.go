package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- DTOs (CSE_014 advanced) ----

type alterarFaseRequest struct {
	Fase string `json:"fase"`
}

type alterarFaseResponse struct {
	FaseAnterior string `json:"faseAnterior"`
	FaseAtual    string `json:"faseAtual"`
	Mensagem     string `json:"mensagem"`
}

type summariesRequest struct {
	IDs []string `json:"ids"`
}

type atualizarPoliticaRequest struct {
	PoliticaConvidados string `json:"politicaConvidados"`
}

type enderecoDTO struct {
	Rua         *string  `json:"rua,omitempty"`
	Numero      *string  `json:"numero,omitempty"`
	Complemento *string  `json:"complemento,omitempty"`
	Bairro      *string  `json:"bairro,omitempty"`
	Cidade      *string  `json:"cidade,omitempty"`
	Estado      *string  `json:"estado,omitempty"`
	Cep         *string  `json:"cep,omitempty"`
	PlaceID     *string  `json:"placeId,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
}

type atualizarDetalhesRequest struct {
	Nome       *string      `json:"nome,omitempty"`
	Tipo       *string      `json:"tipo,omitempty"`
	Descricao  *string      `json:"descricao,omitempty"`
	Local      *string      `json:"local,omitempty"`
	DataInicio *string      `json:"dataInicio,omitempty"`
	DataFim    *string      `json:"dataFim,omitempty"`
	Endereco   *enderecoDTO `json:"endereco,omitempty"`
}

// ---- Handlers ----

func (h *EventHandler) alterarFase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	requester := requesterID(r)
	if requester == "" {
		writeError(w, apierr.BadRequest("header X-Usuario-Id é obrigatório"))
		return
	}
	var req alterarFaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	res, err := h.alterarFaseUC.Execute(r.Context(), portin.AlterarFaseInput{
		EventoID:    id,
		RequesterID: requester,
		FaseDestino: req.Fase,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alterarFaseResponse{
		FaseAnterior: res.FaseAnterior,
		FaseAtual:    res.FaseAtual,
		Mensagem:     res.Mensagem,
	})
}

func (h *EventHandler) uploadImagens(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	requester := requesterID(r)
	if requester == "" {
		writeError(w, apierr.BadRequest("header X-Usuario-Id é obrigatório"))
		return
	}

	const maxMultipartMemory = 32 * 1024 * 1024 // 32MB
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeError(w, apierr.BadRequest("não foi possível interpretar multipart: "+err.Error()))
		return
	}
	form := r.MultipartForm
	if form == nil {
		writeError(w, apierr.BadRequest("multipart vazio"))
		return
	}
	tipo := r.FormValue("tipo")
	files, ok := form.File["files"]
	if !ok || len(files) == 0 {
		// Fallback to "imagens" key for Java parity.
		files = form.File["imagens"]
	}
	if len(files) == 0 {
		writeError(w, apierr.BadRequest("nenhum arquivo enviado (use a chave 'files')"))
		return
	}

	imagens := make([]portin.UploadImagemInput, 0, len(files))
	for _, fh := range files {
		ct := fh.Header.Get("Content-Type")
		f, err := fh.Open()
		if err != nil {
			writeError(w, apierr.BadRequest("não foi possível abrir o arquivo: "+err.Error()))
			return
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			writeError(w, apierr.Internal(err.Error()))
			return
		}
		imagens = append(imagens, portin.UploadImagemInput{
			Filename:    fh.Filename,
			ContentType: ct,
			Size:        fh.Size,
			Data:        data,
		})
	}

	evt, err := h.uploadImagensUC.Execute(r.Context(), portin.UploadImagensInput{
		EventoID:    id,
		RequesterID: requester,
		Tipo:        tipo,
		Imagens:     imagens,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, eventoToResponse(evt))
}

func (h *EventHandler) buscarSummaries(w http.ResponseWriter, r *http.Request) {
	var req summariesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.buscarSummariesUC.Execute(r.Context(), req.IDs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *EventHandler) atualizarPoliticaConvidados(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "eventoId")
	requester := requesterID(r)
	if requester == "" {
		writeError(w, apierr.BadRequest("header X-Usuario-Id é obrigatório"))
		return
	}
	var req atualizarPoliticaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	_, err := h.atualizarPoliticaUC.Execute(r.Context(), portin.AtualizarPoliticaConvidadosInput{
		EventoID:    id,
		RequesterID: requester,
		Politica:    req.PoliticaConvidados,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"politicaConvidados": req.PoliticaConvidados})
}

func (h *EventHandler) atualizarDetalhes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	requester := requesterID(r)
	if requester == "" {
		writeError(w, apierr.BadRequest("header X-Usuario-Id é obrigatório"))
		return
	}
	var req atualizarDetalhesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	dataInicio, err := parseOptionalISODate(req.DataInicio)
	if err != nil {
		writeError(w, apierr.BadRequest("dataInicio inválida: "+err.Error()))
		return
	}
	dataFim, err := parseOptionalISODate(req.DataFim)
	if err != nil {
		writeError(w, apierr.BadRequest("dataFim inválida: "+err.Error()))
		return
	}
	var endIn *portin.EnderecoInput
	if req.Endereco != nil {
		endIn = &portin.EnderecoInput{
			Rua:         req.Endereco.Rua,
			Numero:      req.Endereco.Numero,
			Complemento: req.Endereco.Complemento,
			Bairro:      req.Endereco.Bairro,
			Cidade:      req.Endereco.Cidade,
			Estado:      req.Endereco.Estado,
			Cep:         req.Endereco.Cep,
			PlaceID:     req.Endereco.PlaceID,
			Latitude:    req.Endereco.Latitude,
			Longitude:   req.Endereco.Longitude,
		}
	}
	evt, err := h.atualizarDetalhesUC.Execute(r.Context(), portin.AtualizarDetalhesInput{
		EventoID:    id,
		RequesterID: requester,
		Nome:        req.Nome,
		Tipo:        req.Tipo,
		Descricao:   req.Descricao,
		Local:       req.Local,
		DataInicio:  dataInicio,
		DataFim:     dataFim,
		Endereco:    endIn,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, eventoToResponse(evt))
}

func (h *EventHandler) gerenciarEvento(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	requester := requesterID(r)
	if requester == "" {
		writeError(w, apierr.BadRequest("header X-Usuario-Id é obrigatório"))
		return
	}
	res, err := h.gerenciarUC.Execute(r.Context(), portin.GerenciarEventoInput{
		EventoID:    id,
		RequesterID: requester,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *EventHandler) getEventoCompleto(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := h.getCompletoUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// getPublicInfo — NO AUTH endpoint.
func (h *EventHandler) getPublicInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "eventId")
	res, err := h.getPublicInfoUC.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *EventHandler) joinEvento(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "eventId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	res, err := h.joinEventoUC.Execute(r.Context(), portin.JoinEventoInput{
		EventoID: id,
		UserID:   userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	status := res.HTTPStatusCode
	if status == 0 {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"status":        res.Status,
		"participantId": res.ParticipantID,
	})
}

// ---- helpers ----

// requesterID extracts the actor identity preferring the X-Usuario-Id header
// (Java compatibility) and falling back to the JWT-derived user id.
func requesterID(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Usuario-Id")); v != "" {
		return v
	}
	return middleware.UserIDFromContext(r.Context())
}

// parseOptionalISODate parses optional ISO-8601 strings with or without millis.
func parseOptionalISODate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	formats := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02T15:04:05Z", "2006-01-02"}
	var lastErr error
	for _, f := range formats {
		if t, err := time.Parse(f, *s); err == nil {
			return &t, nil
		} else {
			lastErr = err
		}
	}
	return nil, lastErr
}
