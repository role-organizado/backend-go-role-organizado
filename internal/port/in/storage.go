package in

import (
	"context"
	"io"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/storage"
)

// UploadArquivoInput contains data for uploading a file.
type UploadArquivoInput struct {
	OwnerID      string
	NomeOriginal string
	ContentType  string
	Reader       io.Reader
}

// UploadArquivoUseCase uploads a file and persists its metadata.
type UploadArquivoUseCase interface {
	Execute(ctx context.Context, input UploadArquivoInput) (*storage.Arquivo, error)
}

// DownloadArquivoUseCase retrieves a file stream.
type DownloadArquivoUseCase interface {
	Execute(ctx context.Context, id, userID string) (*storage.Arquivo, io.ReadCloser, error)
}

// DeleteArquivoUseCase deletes a file and its metadata.
type DeleteArquivoUseCase interface {
	Execute(ctx context.Context, id, userID string) error
}
