package storage

import (
	"context"
	"io"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/storage"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// UploadArquivo implements portin.UploadArquivoUseCase.
type UploadArquivo struct {
	repo    portout.ArquivoRepository
	gridfs  portout.GridFSStorage
}

var _ portin.UploadArquivoUseCase = (*UploadArquivo)(nil)

func NewUploadArquivo(repo portout.ArquivoRepository, gridfs portout.GridFSStorage) *UploadArquivo {
	return &UploadArquivo{repo: repo, gridfs: gridfs}
}

func (uc *UploadArquivo) Execute(ctx context.Context, input portin.UploadArquivoInput) (*domain.Arquivo, error) {
	gridFSID, size, err := uc.gridfs.Upload(ctx, input.NomeOriginal, input.ContentType, input.Reader)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}

	a := &domain.Arquivo{
		OwnerID:      input.OwnerID,
		NomeOriginal: input.NomeOriginal,
		ContentType:  input.ContentType,
		Tamanho:      size,
		GridFSID:     gridFSID,
		BucketName:   "comprovantes",
		CriadoEm:    time.Now(),
	}
	return uc.repo.Save(ctx, a)
}

// DownloadArquivo implements portin.DownloadArquivoUseCase.
type DownloadArquivo struct {
	repo   portout.ArquivoRepository
	gridfs portout.GridFSStorage
}

var _ portin.DownloadArquivoUseCase = (*DownloadArquivo)(nil)

func NewDownloadArquivo(repo portout.ArquivoRepository, gridfs portout.GridFSStorage) *DownloadArquivo {
	return &DownloadArquivo{repo: repo, gridfs: gridfs}
}

func (uc *DownloadArquivo) Execute(ctx context.Context, id, userID string) (*domain.Arquivo, io.ReadCloser, error) {
	a, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if !a.IsOwner(userID) {
		return nil, nil, apierr.Forbidden("arquivo não pertence ao usuário autenticado")
	}
	rc, err := uc.gridfs.Download(ctx, a.GridFSID)
	if err != nil {
		return nil, nil, apierr.Internal(err.Error())
	}
	return a, rc, nil
}

// DeleteArquivo implements portin.DeleteArquivoUseCase.
type DeleteArquivo struct {
	repo   portout.ArquivoRepository
	gridfs portout.GridFSStorage
}

var _ portin.DeleteArquivoUseCase = (*DeleteArquivo)(nil)

func NewDeleteArquivo(repo portout.ArquivoRepository, gridfs portout.GridFSStorage) *DeleteArquivo {
	return &DeleteArquivo{repo: repo, gridfs: gridfs}
}

func (uc *DeleteArquivo) Execute(ctx context.Context, id, userID string) error {
	a, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !a.IsOwner(userID) {
		return apierr.Forbidden("arquivo não pertence ao usuário autenticado")
	}
	if err := uc.gridfs.Delete(ctx, a.GridFSID); err != nil {
		return apierr.Internal(err.Error())
	}
	return uc.repo.DeleteByID(ctx, id)
}
