package out

import (
	"context"
	"io"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/storage"
)

// ArquivoRepository handles Arquivo metadata persistence.
type ArquivoRepository interface {
	FindByID(ctx context.Context, id string) (*storage.Arquivo, error)
	Save(ctx context.Context, a *storage.Arquivo) (*storage.Arquivo, error)
	DeleteByID(ctx context.Context, id string) error
}

// GridFSStorage handles binary file storage.
type GridFSStorage interface {
	Upload(ctx context.Context, filename, contentType string, r io.Reader) (gridFSID string, size int64, err error)
	Download(ctx context.Context, gridFSID string) (io.ReadCloser, error)
	Delete(ctx context.Context, gridFSID string) error
}
