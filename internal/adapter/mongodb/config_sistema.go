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

// configSistemaDocument is the MongoDB document for ConfiguracaoSistema.
type configSistemaDocument struct {
	ID        bson.ObjectID  `bson:"_id,omitempty"`
	Chave     string         `bson:"chave"`
	Valor     map[string]any `bson:"valor"`
	Descricao string         `bson:"descricao,omitempty"`
	Ativo     bool           `bson:"ativo"`
	CriadoEm time.Time      `bson:"criado_em"`
	UpdatedAt time.Time      `bson:"updated_at"`
}

// ConfigSistemaRepository implements portout.ConfigSistemaRepository using MongoDB.
type ConfigSistemaRepository struct {
	col *mongo.Collection
}

// NewConfigSistemaRepository creates a ConfigSistemaRepository.
func NewConfigSistemaRepository(client *Client) *ConfigSistemaRepository {
	return &ConfigSistemaRepository{col: client.Collection("configuracao_sistema")}
}

// FindAll returns all system configurations.
func (r *ConfigSistemaRepository) FindAll(ctx context.Context) ([]config.ConfiguracaoSistema, error) {
	cur, err := r.col.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "chave", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []configSistemaDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	result := make([]config.ConfiguracaoSistema, len(docs))
	for i, doc := range docs {
		result[i] = configSistemaFromDoc(doc)
	}
	return result, nil
}

// FindByChave returns a single ConfiguracaoSistema by its unique key.
func (r *ConfigSistemaRepository) FindByChave(ctx context.Context, chave string) (*config.ConfiguracaoSistema, error) {
	var doc configSistemaDocument
	err := r.col.FindOne(ctx, bson.D{{Key: "chave", Value: chave}}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, apierr.NotFound("configuracao_sistema", chave)
	}
	if err != nil {
		return nil, err
	}
	c := configSistemaFromDoc(doc)
	return &c, nil
}

// Save inserts or updates a ConfiguracaoSistema (upsert by chave).
func (r *ConfigSistemaRepository) Save(ctx context.Context, c *config.ConfiguracaoSistema) (*config.ConfiguracaoSistema, error) {
	now := time.Now().UTC()
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "valor", Value: c.Valor},
			{Key: "descricao", Value: c.Descricao},
			{Key: "ativo", Value: c.Ativo},
			{Key: "updated_at", Value: now},
		}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "chave", Value: c.Chave},
			{Key: "criado_em", Value: now},
		}},
	}
	filter := bson.D{{Key: "chave", Value: c.Chave}}
	upsertOpts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, upsertOpts)
	if err != nil {
		return nil, err
	}
	return r.FindByChave(ctx, c.Chave)
}

func configSistemaFromDoc(doc configSistemaDocument) config.ConfiguracaoSistema {
	return config.ConfiguracaoSistema{
		ID:        doc.ID.Hex(),
		Chave:     doc.Chave,
		Valor:     doc.Valor,
		Descricao: doc.Descricao,
		Ativo:     doc.Ativo,
		CriadoEm:  doc.CriadoEm,
		UpdatedAt: doc.UpdatedAt,
	}
}
