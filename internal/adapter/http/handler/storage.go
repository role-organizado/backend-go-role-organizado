package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// StorageHandler handles file upload/download/delete endpoints.
type StorageHandler struct {
	uploadUC   portin.UploadArquivoUseCase
	downloadUC portin.DownloadArquivoUseCase
	deleteUC   portin.DeleteArquivoUseCase
}

// NewStorageHandler creates a new StorageHandler.
func NewStorageHandler(
	uploadUC portin.UploadArquivoUseCase,
	downloadUC portin.DownloadArquivoUseCase,
	deleteUC portin.DeleteArquivoUseCase,
) *StorageHandler {
	return &StorageHandler{
		uploadUC:   uploadUC,
		downloadUC: downloadUC,
		deleteUC:   deleteUC,
	}
}

// RegisterStorageRoutes registers file storage routes under r (already protected by auth middleware).
func (h *StorageHandler) RegisterStorageRoutes(r chi.Router) {
	r.Route("/api/v1/files", func(r chi.Router) {
		r.Post("/upload", h.upload)
		r.Get("/{id}", h.download)
		r.Delete("/{id}", h.deleteFile)
	})
}

// upload handles POST /api/v1/files/upload (multipart/form-data)
func (h *StorageHandler) upload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	// Parse multipart form (32MB limit)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, apierr.BadRequest("falha ao processar formulário"))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, apierr.BadRequest("campo 'file' obrigatório"))
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	result, err := h.uploadUC.Execute(r.Context(), portin.UploadArquivoInput{
		OwnerID:      userID,
		NomeOriginal: header.Filename,
		ContentType:  contentType,
		Reader:       file,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// download handles GET /api/v1/files/{id}
func (h *StorageHandler) download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())

	arquivo, rc, err := h.downloadUC.Execute(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", arquivo.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, arquivo.NomeOriginal))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", arquivo.Tamanho))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, rc) //nolint:errcheck
}

// deleteFile handles DELETE /api/v1/files/{id}
func (h *StorageHandler) deleteFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.UserIDFromContext(r.Context())

	if err := h.deleteUC.Execute(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
