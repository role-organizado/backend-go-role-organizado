package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// SocialHandler handles the Social Features domain HTTP endpoints.
// It covers all 14 operations under /api/v1/eventos/{eventoId}/social.
type SocialHandler struct {
	getUC                  portin.GetSocialFeaturesUseCase
	setDressCodeUC         portin.SetDressCodeUseCase
	removeDressCodeUC      portin.RemoveDressCodeUseCase
	addPlaylistUC          portin.AddPlaylistUseCase
	removePlaylistUC       portin.RemovePlaylistUseCase
	addBringListItemUC     portin.AddBringListItemUseCase
	updateBringListItemUC  portin.UpdateBringListItemUseCase
	removeBringListItemUC  portin.RemoveBringListItemUseCase
	claimBringListItemUC   portin.ClaimBringListItemUseCase
	unclaimBringListItemUC portin.UnclaimBringListItemUseCase
	setCheckinHabilitadoUC portin.SetCheckinHabilitadoUseCase
	doCheckinUC            portin.DoCheckinUseCase
	addAlbumLinkUC         portin.AddAlbumLinkUseCase
	removeAlbumLinkUC      portin.RemoveAlbumLinkUseCase
}

// NewSocialHandler creates a new SocialHandler with all 14 use cases injected.
func NewSocialHandler(
	get portin.GetSocialFeaturesUseCase,
	setDressCode portin.SetDressCodeUseCase,
	removeDressCode portin.RemoveDressCodeUseCase,
	addPlaylist portin.AddPlaylistUseCase,
	removePlaylist portin.RemovePlaylistUseCase,
	addBringListItem portin.AddBringListItemUseCase,
	updateBringListItem portin.UpdateBringListItemUseCase,
	removeBringListItem portin.RemoveBringListItemUseCase,
	claimBringListItem portin.ClaimBringListItemUseCase,
	unclaimBringListItem portin.UnclaimBringListItemUseCase,
	setCheckinHabilitado portin.SetCheckinHabilitadoUseCase,
	doCheckin portin.DoCheckinUseCase,
	addAlbumLink portin.AddAlbumLinkUseCase,
	removeAlbumLink portin.RemoveAlbumLinkUseCase,
) *SocialHandler {
	return &SocialHandler{
		getUC:                  get,
		setDressCodeUC:         setDressCode,
		removeDressCodeUC:      removeDressCode,
		addPlaylistUC:          addPlaylist,
		removePlaylistUC:       removePlaylist,
		addBringListItemUC:     addBringListItem,
		updateBringListItemUC:  updateBringListItem,
		removeBringListItemUC:  removeBringListItem,
		claimBringListItemUC:   claimBringListItem,
		unclaimBringListItemUC: unclaimBringListItem,
		setCheckinHabilitadoUC: setCheckinHabilitado,
		doCheckinUC:            doCheckin,
		addAlbumLinkUC:         addAlbumLink,
		removeAlbumLinkUC:      removeAlbumLink,
	}
}

// RegisterSocialRoutes mounts all 14 social feature routes onto the given chi router.
// Base path: /api/v1/eventos/{eventoId}/social
func (h *SocialHandler) RegisterSocialRoutes(r chi.Router) {
	r.Route("/api/v1/eventos/{eventoId}/social", func(r chi.Router) {
		r.Get("/", h.getSocialFeatures)
		r.Put("/dress-code", h.setDressCode)
		r.Delete("/dress-code", h.removeDressCode)
		r.Post("/playlists", h.addPlaylist)
		r.Delete("/playlists/{playlistId}", h.removePlaylist)
		r.Post("/bring-list", h.addBringListItem)
		r.Put("/bring-list/{itemId}", h.updateBringListItem)
		r.Delete("/bring-list/{itemId}", h.removeBringListItem)
		r.Post("/bring-list/{itemId}/claim", h.claimBringListItem)
		r.Delete("/bring-list/{itemId}/claim", h.unclaimBringListItem)
		r.Put("/checkin/habilitado", h.setCheckinHabilitado)
		r.Post("/checkin", h.doCheckin)
		r.Post("/album-links", h.addAlbumLink)
		r.Delete("/album-links/{linkId}", h.removeAlbumLink)
	})
}

// ---- Request DTOs ----

type setDressCodeRequest struct {
	Tipo              string `json:"tipo"`
	DescricaoTematico string `json:"descricaoTematico"`
}

type addPlaylistRequest struct {
	URL  string `json:"url"`
	Nome string `json:"nome"`
}

type addBringListItemRequest struct {
	Nome       string `json:"nome"`
	Quantidade string `json:"quantidade"`
}

type updateBringListItemRequest struct {
	Nome       string `json:"nome"`
	Quantidade string `json:"quantidade"`
}

type setCheckinHabilitadoRequest struct {
	CheckinHabilitado bool `json:"checkinHabilitado"`
}

type addAlbumLinkRequest struct {
	URL  string `json:"url"`
	Nome string `json:"nome"`
}

// ---- Response DTOs ----

type socialDressCodeResponse struct {
	Tipo              string `json:"tipo"`
	DescricaoTematico string `json:"descricaoTematico,omitempty"`
}

type socialPlaylistLinkResponse struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Nome     string `json:"nome"`
	Provider string `json:"provider"`
	EmbedURL string `json:"embedUrl,omitempty"`
}

type socialBringListItemResponse struct {
	ID            string     `json:"id"`
	Nome          string     `json:"nome"`
	Quantidade    string     `json:"quantidade,omitempty"`
	ClaimedBy     string     `json:"claimedBy,omitempty"`
	ClaimedByNome string     `json:"claimedByNome,omitempty"`
	ClaimedAt     *time.Time `json:"claimedAt,omitempty"`
}

type socialCheckinResponse struct {
	UsuarioID string    `json:"usuarioId"`
	Nome      string    `json:"nome"`
	Timestamp time.Time `json:"timestamp"`
}

type socialAlbumLinkResponse struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Nome          string `json:"nome"`
	Provider      string `json:"provider"`
	AdicionadoPor string `json:"adicionadoPor"`
}

type socialFeaturesResponse struct {
	EventoID          string                        `json:"eventoId"`
	DressCode         *socialDressCodeResponse      `json:"dressCode,omitempty"`
	Playlists         []socialPlaylistLinkResponse  `json:"playlists"`
	BringList         []socialBringListItemResponse `json:"bringList"`
	CheckinHabilitado bool                          `json:"checkinHabilitado"`
	Checkins          []socialCheckinResponse       `json:"checkins"`
	AlbumLinks        []socialAlbumLinkResponse     `json:"albumLinks"`
}

// ---- Mapping helpers ----

func toSocialDressCodeResponse(dc *domain.DressCode) *socialDressCodeResponse {
	if dc == nil {
		return nil
	}
	return &socialDressCodeResponse{
		Tipo:              string(dc.Tipo),
		DescricaoTematico: dc.DescricaoTematico,
	}
}

func toSocialPlaylistLinkResponse(p domain.PlaylistLink) socialPlaylistLinkResponse {
	return socialPlaylistLinkResponse{
		ID:       p.ID,
		URL:      p.URL,
		Nome:     p.Nome,
		Provider: string(p.Provider),
		EmbedURL: p.EmbedURL,
	}
}

func toSocialBringListItemResponse(item domain.BringListItem) socialBringListItemResponse {
	return socialBringListItemResponse{
		ID:            item.ID,
		Nome:          item.Nome,
		Quantidade:    item.Quantidade,
		ClaimedBy:     item.ClaimedBy,
		ClaimedByNome: item.ClaimedByNome,
		ClaimedAt:     item.ClaimedAt,
	}
}

func toSocialCheckinResponse(c domain.Checkin) socialCheckinResponse {
	return socialCheckinResponse{
		UsuarioID: c.UsuarioID,
		Nome:      c.Nome,
		Timestamp: c.Timestamp,
	}
}

func toSocialAlbumLinkResponse(link domain.AlbumLink) socialAlbumLinkResponse {
	return socialAlbumLinkResponse{
		ID:            link.ID,
		URL:           link.URL,
		Nome:          link.Nome,
		Provider:      string(link.Provider),
		AdicionadoPor: link.AdicionadoPor,
	}
}

func toSocialFeaturesResponse(sf *domain.EventoSocialFeatures) socialFeaturesResponse {
	playlists := make([]socialPlaylistLinkResponse, len(sf.Playlists))
	for i, p := range sf.Playlists {
		playlists[i] = toSocialPlaylistLinkResponse(p)
	}
	bringList := make([]socialBringListItemResponse, len(sf.BringList))
	for i, item := range sf.BringList {
		bringList[i] = toSocialBringListItemResponse(item)
	}
	checkins := make([]socialCheckinResponse, len(sf.Checkins))
	for i, c := range sf.Checkins {
		checkins[i] = toSocialCheckinResponse(c)
	}
	albumLinks := make([]socialAlbumLinkResponse, len(sf.AlbumLinks))
	for i, link := range sf.AlbumLinks {
		albumLinks[i] = toSocialAlbumLinkResponse(link)
	}
	return socialFeaturesResponse{
		EventoID:          sf.EventoID,
		DressCode:         toSocialDressCodeResponse(sf.DressCode),
		Playlists:         playlists,
		BringList:         bringList,
		CheckinHabilitado: sf.CheckinHabilitado,
		Checkins:          checkins,
		AlbumLinks:        albumLinks,
	}
}

// ---- Handlers ----

// getSocialFeatures handles GET /api/v1/eventos/{eventoId}/social/
// Returns the full social features document (200) or an empty structure when no doc exists.
func (h *SocialHandler) getSocialFeatures(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	sf, err := h.getUC.Execute(r.Context(), portin.GetSocialFeaturesInput{
		EventoID:  eventoID,
		UsuarioID: userID,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSocialFeaturesResponse(sf))
}

// setDressCode handles PUT /api/v1/eventos/{eventoId}/social/dress-code
// Sets or updates the dress code (organizador only). Returns 200 with the updated document.
func (h *SocialHandler) setDressCode(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req setDressCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	sf, err := h.setDressCodeUC.Execute(r.Context(), portin.SetDressCodeInput{
		EventoID:          eventoID,
		UsuarioID:         userID,
		Tipo:              req.Tipo,
		DescricaoTematico: req.DescricaoTematico,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSocialFeaturesResponse(sf))
}

// removeDressCode handles DELETE /api/v1/eventos/{eventoId}/social/dress-code
// Removes the dress code (organizador only). Idempotent. Returns 204.
func (h *SocialHandler) removeDressCode(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	if err := h.removeDressCodeUC.Execute(r.Context(), portin.RemoveDressCodeInput{
		EventoID:  eventoID,
		UsuarioID: userID,
	}); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// addPlaylist handles POST /api/v1/eventos/{eventoId}/social/playlists
// Adds a playlist link (organizador only, max 3). Returns 201 with the created playlist.
func (h *SocialHandler) addPlaylist(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req addPlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	playlist, err := h.addPlaylistUC.Execute(r.Context(), portin.AddPlaylistInput{
		EventoID:  eventoID,
		UsuarioID: userID,
		URL:       req.URL,
		Nome:      req.Nome,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toSocialPlaylistLinkResponse(*playlist))
}

// removePlaylist handles DELETE /api/v1/eventos/{eventoId}/social/playlists/{playlistId}
// Removes a playlist link (organizador only). Idempotent. Returns 204.
func (h *SocialHandler) removePlaylist(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	playlistID := chi.URLParam(r, "playlistId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	if err := h.removePlaylistUC.Execute(r.Context(), portin.RemovePlaylistInput{
		EventoID:   eventoID,
		PlaylistID: playlistID,
		UsuarioID:  userID,
	}); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// addBringListItem handles POST /api/v1/eventos/{eventoId}/social/bring-list
// Adds a bring list item (organizador only). Returns 201 with the created item.
func (h *SocialHandler) addBringListItem(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req addBringListItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	item, err := h.addBringListItemUC.Execute(r.Context(), portin.AddBringListItemInput{
		EventoID:   eventoID,
		UsuarioID:  userID,
		Nome:       req.Nome,
		Quantidade: req.Quantidade,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toSocialBringListItemResponse(*item))
}

// updateBringListItem handles PUT /api/v1/eventos/{eventoId}/social/bring-list/{itemId}
// Updates a bring list item's nome/quantidade (organizador only). Returns 202.
func (h *SocialHandler) updateBringListItem(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	itemID := chi.URLParam(r, "itemId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req updateBringListItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	if err := h.updateBringListItemUC.Execute(r.Context(), portin.UpdateBringListItemInput{
		EventoID:   eventoID,
		ItemID:     itemID,
		UsuarioID:  userID,
		Nome:       req.Nome,
		Quantidade: req.Quantidade,
	}); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// removeBringListItem handles DELETE /api/v1/eventos/{eventoId}/social/bring-list/{itemId}
// Removes a bring list item (organizador only). Idempotent. Returns 204.
func (h *SocialHandler) removeBringListItem(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	itemID := chi.URLParam(r, "itemId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	if err := h.removeBringListItemUC.Execute(r.Context(), portin.RemoveBringListItemInput{
		EventoID:  eventoID,
		ItemID:    itemID,
		UsuarioID: userID,
	}); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// claimBringListItem handles POST /api/v1/eventos/{eventoId}/social/bring-list/{itemId}/claim
// Atomically claims a bring list item for the authenticated user. Returns 202.
// Returns 409 BRING_LIST_ITEM_ALREADY_CLAIMED if the item is already claimed.
func (h *SocialHandler) claimBringListItem(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	itemID := chi.URLParam(r, "itemId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	// User name is sourced from JWT claims, never from the request body.
	usuarioNome := ""
	if claims := middleware.ClaimsFromContext(r.Context()); claims != nil {
		usuarioNome = claims.Nome
	}

	if err := h.claimBringListItemUC.Execute(r.Context(), portin.ClaimBringListItemInput{
		EventoID:    eventoID,
		ItemID:      itemID,
		UsuarioID:   userID,
		UsuarioNome: usuarioNome,
	}); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// unclaimBringListItem handles DELETE /api/v1/eventos/{eventoId}/social/bring-list/{itemId}/claim
// Releases the claim on a bring list item. Idempotent. Returns 204.
func (h *SocialHandler) unclaimBringListItem(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	itemID := chi.URLParam(r, "itemId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	if err := h.unclaimBringListItemUC.Execute(r.Context(), portin.UnclaimBringListItemInput{
		EventoID:  eventoID,
		ItemID:    itemID,
		UsuarioID: userID,
	}); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// setCheckinHabilitado handles PUT /api/v1/eventos/{eventoId}/social/checkin/habilitado
// Enables or disables check-in for the event (organizador only). Returns 202.
func (h *SocialHandler) setCheckinHabilitado(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req setCheckinHabilitadoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	if err := h.setCheckinHabilitadoUC.Execute(r.Context(), portin.SetCheckinHabilitadoInput{
		EventoID:   eventoID,
		UsuarioID:  userID,
		Habilitado: req.CheckinHabilitado,
	}); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// doCheckin handles POST /api/v1/eventos/{eventoId}/social/checkin
// Registers the authenticated participant's check-in. Returns 200 with CheckinResponse.
// Returns 409 CHECKIN_NOT_ENABLED when check-in is disabled.
// Returns 409 CHECKIN_ALREADY_REGISTERED when the user already checked in.
func (h *SocialHandler) doCheckin(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	// User name is sourced from JWT claims, never from the request body.
	usuarioNome := ""
	if claims := middleware.ClaimsFromContext(r.Context()); claims != nil {
		usuarioNome = claims.Nome
	}

	checkin, err := h.doCheckinUC.Execute(r.Context(), portin.DoCheckinInput{
		EventoID:    eventoID,
		UsuarioID:   userID,
		UsuarioNome: usuarioNome,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSocialCheckinResponse(*checkin))
}

// addAlbumLink handles POST /api/v1/eventos/{eventoId}/social/album-links
// Adds a photo album link (participante confirmado or organizador, max 5). Returns 201.
func (h *SocialHandler) addAlbumLink(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	var req addAlbumLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo de requisição inválido"))
		return
	}

	link, err := h.addAlbumLinkUC.Execute(r.Context(), portin.AddAlbumLinkInput{
		EventoID:  eventoID,
		UsuarioID: userID,
		URL:       req.URL,
		Nome:      req.Nome,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toSocialAlbumLinkResponse(*link))
}

// removeAlbumLink handles DELETE /api/v1/eventos/{eventoId}/social/album-links/{linkId}
// Removes a photo album link (organizador only). Idempotent. Returns 204.
func (h *SocialHandler) removeAlbumLink(w http.ResponseWriter, r *http.Request) {
	eventoID := chi.URLParam(r, "eventoId")
	linkID := chi.URLParam(r, "linkId")
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}

	if err := h.removeAlbumLinkUC.Execute(r.Context(), portin.RemoveAlbumLinkInput{
		EventoID:  eventoID,
		LinkID:    linkID,
		UsuarioID: userID,
	}); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
