package mongodb

import (
	"bytes"
	"context"
	"io"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/storage"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// arquivoDocument is the MongoDB document for Arquivo metadata.
type arquivoDocument struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	OwnerID      string        `bson:"owner_id"`
	NomeOriginal string        `bson:"nome_original"`
	ContentType  string        `bson:"content_type"`
	Tamanho      int64         `bson:"tamanho"`
	GridFSID     string        `bson:"gridfs_id"`
	BucketName   string        `bson:"bucket_name"`
	CriadoEm    time.Time     `bson:"criado_em"`
}

// ArquivoMongoRepository implements portout.ArquivoRepository.
type ArquivoMongoRepository struct {
	col *mongo.Collection
}

// NewArquivoRepository creates a new ArquivoMongoRepository.
func NewArquivoRepository(client *Client) *ArquivoMongoRepository {
	return &ArquivoMongoRepository{col: client.Collection("arquivos_metadata")}
}

func (r *ArquivoMongoRepository) FindByID(ctx context.Context, id string) (*domain.Arquivo, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apierr.NotFound("arquivo", id)
	}
	var doc arquivoDocument
	if err := r.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("arquivo", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return &domain.Arquivo{
		ID:           doc.ID.Hex(),
		OwnerID:      doc.OwnerID,
		NomeOriginal: doc.NomeOriginal,
		ContentType:  doc.ContentType,
		Tamanho:      doc.Tamanho,
		GridFSID:     doc.GridFSID,
		BucketName:   doc.BucketName,
		CriadoEm:    doc.CriadoEm,
	}, nil
}

func (r *ArquivoMongoRepository) Save(ctx context.Context, a *domain.Arquivo) (*domain.Arquivo, error) {
	doc := arquivoDocument{
		ID:           bson.NewObjectID(),
		OwnerID:      a.OwnerID,
		NomeOriginal: a.NomeOriginal,
		ContentType:  a.ContentType,
		Tamanho:      a.Tamanho,
		GridFSID:     a.GridFSID,
		BucketName:   a.BucketName,
		CriadoEm:    a.CriadoEm,
	}
	_, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	a.ID = doc.ID.Hex()
	return a, nil
}

func (r *ArquivoMongoRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.NotFound("arquivo", id)
	}
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("arquivo", id)
	}
	return nil
}

// GridFSStorageAdapter implements portout.GridFSStorage using MongoDB GridFS.
type GridFSStorageAdapter struct {
	bucket *mongo.GridFSBucket
}

// NewGridFSStorageAdapter creates a new GridFSStorageAdapter using the "comprovantes" bucket.
func NewGridFSStorageAdapter(client *Client) *GridFSStorageAdapter {
	bucket := client.DB().GridFSBucket(options.GridFSBucket().SetName("comprovantes"))
	return &GridFSStorageAdapter{bucket: bucket}
}

func (g *GridFSStorageAdapter) Upload(ctx context.Context, filename, contentType string, r io.Reader) (string, int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", 0, err
	}

	uploadOpts := options.GridFSUpload().SetMetadata(bson.M{"content_type": contentType})
	fileID, err := g.bucket.UploadFromStream(ctx, filename, bytes.NewReader(data), uploadOpts)
	if err != nil {
		return "", 0, err
	}

	return fileID.Hex(), int64(len(data)), nil
}

func (g *GridFSStorageAdapter) Download(ctx context.Context, gridFSID string) (io.ReadCloser, error) {
	oid, err := bson.ObjectIDFromHex(gridFSID)
	if err != nil {
		return nil, apierr.NotFound("arquivo", gridFSID)
	}
	stream, err := g.bucket.OpenDownloadStream(ctx, oid)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return stream, nil
}

func (g *GridFSStorageAdapter) Delete(ctx context.Context, gridFSID string) error {
	oid, err := bson.ObjectIDFromHex(gridFSID)
	if err != nil {
		return apierr.NotFound("arquivo", gridFSID)
	}
	if err := g.bucket.Delete(ctx, oid); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}
