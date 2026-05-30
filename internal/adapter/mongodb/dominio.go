package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// dominioDocument is the MongoDB document representation of a Dominio.
type dominioDocument struct {
	ID        bson.ObjectID          `bson:"_id,omitempty"`
	Categoria string                 `bson:"categoria"`
	Chave     string                 `bson:"chave"`
	Valor     string                 `bson:"valor"`
	Descricao string                 `bson:"descricao,omitempty"`
	Icone     string                 `bson:"icone,omitempty"`
	Ordem     int                    `bson:"ordem"`
	Ativo     bool                   `bson:"ativo"`
	Metadata  map[string]any         `bson:"metadata,omitempty"`
	CriadoEm time.Time              `bson:"criado_em"`
	UpdatedAt time.Time              `bson:"updated_at"`
}

// DominioRepository implements portout.DominioRepository using MongoDB.
type DominioRepository struct {
	col *mongo.Collection
}

// NewDominioRepository creates a DominioRepository backed by the given Client.
func NewDominioRepository(client *Client) *DominioRepository {
	return &DominioRepository{col: client.Collection("dominios")}
}

// FindAll returns all Dominio entries ordered by categoria asc, ordem asc.
func (r *DominioRepository) FindAll(ctx context.Context) ([]config.Dominio, error) {
	opts := options.Find().SetSort(bson.D{{Key: "categoria", Value: 1}, {Key: "ordem", Value: 1}})
	cur, err := r.col.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, err
	}
	return decodeDominios(ctx, cur)
}

// FindByCategoria returns Dominio entries matching the given categoria.
func (r *DominioRepository) FindByCategoria(ctx context.Context, categoria string) ([]config.Dominio, error) {
	opts := options.Find().SetSort(bson.D{{Key: "ordem", Value: 1}})
	cur, err := r.col.Find(ctx, bson.D{{Key: "categoria", Value: categoria}}, opts)
	if err != nil {
		return nil, err
	}
	return decodeDominios(ctx, cur)
}

// FindByCategoriaAndAtivo returns Dominio entries matching categoria and ativo.
func (r *DominioRepository) FindByCategoriaAndAtivo(ctx context.Context, categoria string, ativo bool) ([]config.Dominio, error) {
	filter := bson.D{{Key: "categoria", Value: categoria}, {Key: "ativo", Value: ativo}}
	opts := options.Find().SetSort(bson.D{{Key: "ordem", Value: 1}})
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	return decodeDominios(ctx, cur)
}

// FindByCategoriaAndChave returns a single Dominio matching categoria+chave.
func (r *DominioRepository) FindByCategoriaAndChave(ctx context.Context, categoria, chave string) (*config.Dominio, error) {
	filter := bson.D{{Key: "categoria", Value: categoria}, {Key: "chave", Value: chave}}
	var doc dominioDocument
	err := r.col.FindOne(ctx, filter).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, apierr.NotFound("dominio", categoria+"/"+chave)
	}
	if err != nil {
		return nil, err
	}
	d := dominioFromDoc(doc)
	return &d, nil
}

// FindByID returns a Dominio by its ObjectID hex string.
func (r *DominioRepository) FindByID(ctx context.Context, id string) (*config.Dominio, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apierr.BadRequest("id inválido: " + id)
	}
	var doc dominioDocument
	err = r.col.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, apierr.NotFound("dominio", id)
	}
	if err != nil {
		return nil, err
	}
	d := dominioFromDoc(doc)
	return &d, nil
}

// Save inserts or replaces a Dominio (upsert by categoria+chave when no ID).
func (r *DominioRepository) Save(ctx context.Context, d *config.Dominio) (*config.Dominio, error) {
	now := time.Now().UTC()
	doc := dominioDocument{
		Categoria: d.Categoria,
		Chave:     d.Chave,
		Valor:     d.Valor,
		Descricao: d.Descricao,
		Icone:     d.Icone,
		Ordem:     d.Ordem,
		Ativo:     d.Ativo,
		Metadata:  d.Metadata,
		UpdatedAt: now,
	}

	if d.ID != "" {
		oid, err := bson.ObjectIDFromHex(d.ID)
		if err != nil {
			return nil, apierr.BadRequest("id inválido: " + d.ID)
		}
		doc.ID = oid
		doc.CriadoEm = d.CriadoEm
	} else {
		doc.ID = bson.NewObjectID()
		doc.CriadoEm = now
	}

	filter := bson.D{{Key: "_id", Value: doc.ID}}
	upsertOpts := options.Replace().SetUpsert(true)
	if _, err := r.col.ReplaceOne(ctx, filter, doc, upsertOpts); err != nil {
		return nil, err
	}
	saved := dominioFromDoc(doc)
	return &saved, nil
}

// DeleteByID removes a Dominio by its ObjectID hex string.
func (r *DominioRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.BadRequest("id inválido: " + id)
	}
	res, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("dominio", id)
	}
	return nil
}

// ---- helpers ----

func decodeDominios(ctx context.Context, cur *mongo.Cursor) ([]config.Dominio, error) {
	defer cur.Close(ctx)
	var docs []dominioDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	result := make([]config.Dominio, len(docs))
	for i, doc := range docs {
		result[i] = dominioFromDoc(doc)
	}
	return result, nil
}

func dominioFromDoc(doc dominioDocument) config.Dominio {
	return config.Dominio{
		ID:        doc.ID.Hex(),
		Categoria: doc.Categoria,
		Chave:     doc.Chave,
		Valor:     doc.Valor,
		Descricao: doc.Descricao,
		Icone:     doc.Icone,
		Ordem:     doc.Ordem,
		Ativo:     doc.Ativo,
		Metadata:  doc.Metadata,
		CriadoEm:  doc.CriadoEm,
		UpdatedAt: doc.UpdatedAt,
	}
}
