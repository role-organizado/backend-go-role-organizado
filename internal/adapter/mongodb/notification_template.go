package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notificationtemplate"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// notificationTemplateDocument is the MongoDB representation of a NotificationTemplate.
type notificationTemplateDocument struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	Nome             string        `bson:"nome"`
	Tipo             string        `bson:"tipo"`
	Categoria        string        `bson:"categoria"`
	Assunto          string        `bson:"assunto"`
	Corpo            string        `bson:"corpo"`
	VariaveisEsperadas []string    `bson:"variaveis_esperadas,omitempty"`
	Ativo            bool          `bson:"ativo"`
	CriadoEm        time.Time     `bson:"criado_em"`
	AtualizadoEm    time.Time     `bson:"atualizado_em"`
}

func notifTemplateDocToDomain(doc notificationTemplateDocument) *domain.NotificationTemplate {
	return &domain.NotificationTemplate{
		ID:               doc.ID.Hex(),
		Nome:             doc.Nome,
		Tipo:             domain.TemplateType(doc.Tipo),
		Categoria:        domain.TemplateCategoria(doc.Categoria),
		Assunto:          doc.Assunto,
		Corpo:            doc.Corpo,
		VariaveisEsperadas: doc.VariaveisEsperadas,
		Ativo:            doc.Ativo,
		CriadoEm:        doc.CriadoEm,
		AtualizadoEm:    doc.AtualizadoEm,
	}
}

// NotificationTemplateMongoRepository implements portout.NotificationTemplateRepository.
type NotificationTemplateMongoRepository struct {
	col *mongo.Collection
}

// NewNotificationTemplateRepository creates a new NotificationTemplateMongoRepository.
func NewNotificationTemplateRepository(client *Client) *NotificationTemplateMongoRepository {
	return &NotificationTemplateMongoRepository{
		col: client.Collection("notification_templates"),
	}
}

func (r *NotificationTemplateMongoRepository) Save(ctx context.Context, t *domain.NotificationTemplate) (*domain.NotificationTemplate, error) {
	doc := notificationTemplateDocument{
		ID:               bson.NewObjectID(),
		Nome:             t.Nome,
		Tipo:             string(t.Tipo),
		Categoria:        string(t.Categoria),
		Assunto:          t.Assunto,
		Corpo:            t.Corpo,
		VariaveisEsperadas: t.VariaveisEsperadas,
		Ativo:            t.Ativo,
		CriadoEm:        t.CriadoEm,
		AtualizadoEm:    t.AtualizadoEm,
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	t.ID = doc.ID.Hex()
	return t, nil
}

func (r *NotificationTemplateMongoRepository) FindByID(ctx context.Context, id string) (*domain.NotificationTemplate, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apierr.NotFound("notification_template", id)
	}
	var doc notificationTemplateDocument
	if err := r.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("notification_template", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return notifTemplateDocToDomain(doc), nil
}

func (r *NotificationTemplateMongoRepository) FindAll(ctx context.Context, page, pageSize int) ([]domain.NotificationTemplate, int64, error) {
	filter := bson.M{}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}

	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSort(bson.D{{Key: "criado_em", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(pageSize))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)

	var docs []notificationTemplateDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}

	result := make([]domain.NotificationTemplate, len(docs))
	for i, d := range docs {
		result[i] = *notifTemplateDocToDomain(d)
	}
	return result, total, nil
}

func (r *NotificationTemplateMongoRepository) Update(ctx context.Context, t *domain.NotificationTemplate) (*domain.NotificationTemplate, error) {
	oid, err := bson.ObjectIDFromHex(t.ID)
	if err != nil {
		return nil, apierr.NotFound("notification_template", t.ID)
	}
	doc := notificationTemplateDocument{
		ID:               oid,
		Nome:             t.Nome,
		Tipo:             string(t.Tipo),
		Categoria:        string(t.Categoria),
		Assunto:          t.Assunto,
		Corpo:            t.Corpo,
		VariaveisEsperadas: t.VariaveisEsperadas,
		Ativo:            t.Ativo,
		CriadoEm:        t.CriadoEm,
		AtualizadoEm:    t.AtualizadoEm,
	}
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": oid}, doc, options.Replace().SetUpsert(false))
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return nil, apierr.NotFound("notification_template", t.ID)
	}
	return t, nil
}

func (r *NotificationTemplateMongoRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.NotFound("notification_template", id)
	}
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("notification_template", id)
	}
	return nil
}

func (r *NotificationTemplateMongoRepository) FindByType(ctx context.Context, tipo domain.TemplateType) (*domain.NotificationTemplate, error) {
	var doc notificationTemplateDocument
	if err := r.col.FindOne(ctx, bson.M{"tipo": string(tipo)}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFoundMsg("nenhum template encontrado para o tipo " + string(tipo))
		}
		return nil, apierr.Internal(err.Error())
	}
	return notifTemplateDocToDomain(doc), nil
}

func (r *NotificationTemplateMongoRepository) FindByCategoria(ctx context.Context, categoria domain.TemplateCategoria, page, pageSize int) ([]domain.NotificationTemplate, int64, error) {
	filter := bson.M{"categoria": string(categoria)}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}

	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSort(bson.D{{Key: "criado_em", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(pageSize))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)

	var docs []notificationTemplateDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}

	result := make([]domain.NotificationTemplate, len(docs))
	for i, d := range docs {
		result[i] = *notifTemplateDocToDomain(d)
	}
	return result, total, nil
}
