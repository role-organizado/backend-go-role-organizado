package storage_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/storage"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucstorage "github.com/role-organizado/backend-go-role-organizado/internal/usecase/storage"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// --- Mocks ---

type mockArquivoRepo struct{ mock.Mock }

func (m *mockArquivoRepo) FindByID(ctx context.Context, id string) (*domain.Arquivo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Arquivo), args.Error(1)
}

func (m *mockArquivoRepo) Save(ctx context.Context, a *domain.Arquivo) (*domain.Arquivo, error) {
	args := m.Called(ctx, a)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Arquivo), args.Error(1)
}

func (m *mockArquivoRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockGridFS struct{ mock.Mock }

func (m *mockGridFS) Upload(ctx context.Context, filename, contentType string, r io.Reader) (string, int64, error) {
	args := m.Called(ctx, filename, contentType, r)
	return args.String(0), int64(args.Int(1)), args.Error(2)
}

func (m *mockGridFS) Download(ctx context.Context, gridFSID string) (io.ReadCloser, error) {
	args := m.Called(ctx, gridFSID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *mockGridFS) Delete(ctx context.Context, gridFSID string) error {
	args := m.Called(ctx, gridFSID)
	return args.Error(0)
}

// --- UploadArquivo Tests ---

func TestUploadArquivo_Success(t *testing.T) {
	repo := new(mockArquivoRepo)
	gridfs := new(mockGridFS)
	uc := ucstorage.NewUploadArquivo(repo, gridfs)

	ctx := context.Background()
	reader := strings.NewReader("file content")

	gridfs.On("Upload", ctx, "receipt.pdf", "application/pdf", reader).
		Return("gridfs-id-123", 12, nil)

	expectedArquivo := &domain.Arquivo{
		ID:           "arquivo-id-1",
		OwnerID:      "user-1",
		NomeOriginal: "receipt.pdf",
		ContentType:  "application/pdf",
		Tamanho:      12,
		GridFSID:     "gridfs-id-123",
		BucketName:   "comprovantes",
		CriadoEm:    time.Now(),
	}
	repo.On("Save", ctx, mock.AnythingOfType("*storage.Arquivo")).Return(expectedArquivo, nil)

	result, err := uc.Execute(ctx, portin.UploadArquivoInput{
		OwnerID:      "user-1",
		NomeOriginal: "receipt.pdf",
		ContentType:  "application/pdf",
		Reader:       reader,
	})

	assert.NoError(t, err)
	assert.Equal(t, "arquivo-id-1", result.ID)
	assert.Equal(t, "comprovantes", result.BucketName)
	repo.AssertExpectations(t)
	gridfs.AssertExpectations(t)
}

func TestUploadArquivo_GridFSError(t *testing.T) {
	repo := new(mockArquivoRepo)
	gridfs := new(mockGridFS)
	uc := ucstorage.NewUploadArquivo(repo, gridfs)

	ctx := context.Background()
	reader := strings.NewReader("data")

	gridfs.On("Upload", ctx, "file.pdf", "application/pdf", reader).
		Return("", 0, assert.AnError)

	_, err := uc.Execute(ctx, portin.UploadArquivoInput{
		OwnerID:      "user-1",
		NomeOriginal: "file.pdf",
		ContentType:  "application/pdf",
		Reader:       reader,
	})

	assert.Error(t, err)
	repo.AssertNotCalled(t, "Save")
}

// --- DownloadArquivo Tests ---

func TestDownloadArquivo_Success(t *testing.T) {
	repo := new(mockArquivoRepo)
	gridfs := new(mockGridFS)
	uc := ucstorage.NewDownloadArquivo(repo, gridfs)

	ctx := context.Background()
	arquivo := &domain.Arquivo{
		ID:       "arquivo-id-1",
		OwnerID:  "user-1",
		GridFSID: "gridfs-id-123",
	}
	repo.On("FindByID", ctx, "arquivo-id-1").Return(arquivo, nil)

	rc := io.NopCloser(bytes.NewReader([]byte("file content")))
	gridfs.On("Download", ctx, "gridfs-id-123").Return(rc, nil)

	resultArquivo, resultRC, err := uc.Execute(ctx, "arquivo-id-1", "user-1")

	assert.NoError(t, err)
	assert.Equal(t, arquivo, resultArquivo)
	assert.NotNil(t, resultRC)
	repo.AssertExpectations(t)
	gridfs.AssertExpectations(t)
}

func TestDownloadArquivo_NotFound(t *testing.T) {
	repo := new(mockArquivoRepo)
	gridfs := new(mockGridFS)
	uc := ucstorage.NewDownloadArquivo(repo, gridfs)

	ctx := context.Background()
	repo.On("FindByID", ctx, "bad-id").Return(nil, apierr.NotFound("arquivo", "bad-id"))

	_, _, err := uc.Execute(ctx, "bad-id", "user-1")

	assert.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 404, ae.Status)
}

func TestDownloadArquivo_Forbidden(t *testing.T) {
	repo := new(mockArquivoRepo)
	gridfs := new(mockGridFS)
	uc := ucstorage.NewDownloadArquivo(repo, gridfs)

	ctx := context.Background()
	arquivo := &domain.Arquivo{
		ID:      "arquivo-id-1",
		OwnerID: "owner-user",
	}
	repo.On("FindByID", ctx, "arquivo-id-1").Return(arquivo, nil)

	_, _, err := uc.Execute(ctx, "arquivo-id-1", "other-user")

	assert.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 403, ae.Status)
	gridfs.AssertNotCalled(t, "Download")
}

// --- DeleteArquivo Tests ---

func TestDeleteArquivo_Success(t *testing.T) {
	repo := new(mockArquivoRepo)
	gridfs := new(mockGridFS)
	uc := ucstorage.NewDeleteArquivo(repo, gridfs)

	ctx := context.Background()
	arquivo := &domain.Arquivo{
		ID:       "arquivo-id-1",
		OwnerID:  "user-1",
		GridFSID: "gridfs-id-123",
	}
	repo.On("FindByID", ctx, "arquivo-id-1").Return(arquivo, nil)
	gridfs.On("Delete", ctx, "gridfs-id-123").Return(nil)
	repo.On("DeleteByID", ctx, "arquivo-id-1").Return(nil)

	err := uc.Execute(ctx, "arquivo-id-1", "user-1")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	gridfs.AssertExpectations(t)
}

func TestDeleteArquivo_Forbidden(t *testing.T) {
	repo := new(mockArquivoRepo)
	gridfs := new(mockGridFS)
	uc := ucstorage.NewDeleteArquivo(repo, gridfs)

	ctx := context.Background()
	arquivo := &domain.Arquivo{
		ID:      "arquivo-id-1",
		OwnerID: "owner-user",
	}
	repo.On("FindByID", ctx, "arquivo-id-1").Return(arquivo, nil)

	err := uc.Execute(ctx, "arquivo-id-1", "attacker")

	assert.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 403, ae.Status)
	gridfs.AssertNotCalled(t, "Delete")
	repo.AssertNotCalled(t, "DeleteByID")
}

func TestDeleteArquivo_NotFound(t *testing.T) {
	repo := new(mockArquivoRepo)
	gridfs := new(mockGridFS)
	uc := ucstorage.NewDeleteArquivo(repo, gridfs)

	ctx := context.Background()
	repo.On("FindByID", ctx, "missing-id").Return(nil, apierr.NotFound("arquivo", "missing-id"))

	err := uc.Execute(ctx, "missing-id", "user-1")

	assert.Error(t, err)
	ae := err.(*apierr.APIError)
	assert.Equal(t, 404, ae.Status)
}
