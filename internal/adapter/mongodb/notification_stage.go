package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	notification "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// notificationStageDocument is the MongoDB representation of a stage-template row.
// Stages live in the same `notification_templates` collection as plain templates
// (Java parity) but carry the chave/canal/metadados fields used by the
// STAGE__<KEY>__<EVENT>__<CHANNEL>__L<N> convention.
type notificationStageDocument struct {
	ID           bson.ObjectID  `bson:"_id,omitempty"`
	Chave        string         `bson:"chave"`
	Canal        string         `bson:"canal"`
	Nome         string         `bson:"nome"`
	Assunto      string         `bson:"assunto,omitempty"`
	Corpo        string         `bson:"corpo"`
	Variaveis    []string       `bson:"variaveis,omitempty"`
	Metadados    map[string]any `bson:"metadados,omitempty"`
	Ativo        bool           `bson:"ativo"`
	CriadoEm     time.Time      `bson:"criado_em"`
	AtualizadoEm time.Time      `bson:"atualizado_em"`
}

func stageDocToDomain(doc notificationStageDocument) notification.NotificationStage {
	return notification.NotificationStage{
		ID:           doc.ID.Hex(),
		Chave:        doc.Chave,
		Canal:        notification.NotificationChannel(doc.Canal),
		Nome:         doc.Nome,
		Assunto:      doc.Assunto,
		Corpo:        doc.Corpo,
		Variaveis:    doc.Variaveis,
		Metadados:    doc.Metadados,
		Ativo:        doc.Ativo,
		CriadoEm:     doc.CriadoEm,
		AtualizadoEm: doc.AtualizadoEm,
	}
}

// NotificationStageMongoRepository implements portout.NotificationStageRepository.
type NotificationStageMongoRepository struct {
	col *mongo.Collection
}

// NewNotificationStageRepository creates a repository over the notification_templates collection.
func NewNotificationStageRepository(client *Client) *NotificationStageMongoRepository {
	return &NotificationStageMongoRepository{
		col: client.Collection("notification_templates"),
	}
}

// FindAll returns every document in the collection. Non-stage templates are
// tolerated and later filtered out by the use case via the key codec.
func (r *NotificationStageMongoRepository) FindAll(ctx context.Context) ([]notification.NotificationStage, error) {
	cursor, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)

	var docs []notificationStageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, apierr.Internal(err.Error())
	}

	result := make([]notification.NotificationStage, 0, len(docs))
	for _, d := range docs {
		if d.Chave == "" {
			continue // plain template without a stage key — skip
		}
		result = append(result, stageDocToDomain(d))
	}
	return result, nil
}

// Save inserts a new stage-template row and stamps the generated id.
func (r *NotificationStageMongoRepository) Save(ctx context.Context, s *notification.NotificationStage) (*notification.NotificationStage, error) {
	doc := notificationStageDocument{
		ID:           bson.NewObjectID(),
		Chave:        s.Chave,
		Canal:        string(s.Canal),
		Nome:         s.Nome,
		Assunto:      s.Assunto,
		Corpo:        s.Corpo,
		Variaveis:    s.Variaveis,
		Metadados:    s.Metadados,
		Ativo:        s.Ativo,
		CriadoEm:     s.CriadoEm,
		AtualizadoEm: s.AtualizadoEm,
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	s.ID = doc.ID.Hex()
	return s, nil
}

// DeleteByID removes a stage-template row by its hex object id.
func (r *NotificationStageMongoRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.NotFound("notification_stage", id)
	}
	if _, err := r.col.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}
