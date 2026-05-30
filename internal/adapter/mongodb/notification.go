package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// notificacaoDocument is the MongoDB document for Notificacao.
type notificacaoDocument struct {
	ID        bson.ObjectID     `bson:"_id,omitempty"`
	UsuarioID string            `bson:"usuario_id"`
	Tipo      string            `bson:"tipo"`
	Status    string            `bson:"status"`
	Titulo    string            `bson:"titulo"`
	Mensagem  string            `bson:"mensagem"`
	Dados     map[string]string `bson:"dados,omitempty"`
	CriadoEm time.Time         `bson:"criado_em"`
	LidoEm   *time.Time        `bson:"lido_em,omitempty"`
}

func notDocToDomain(doc notificacaoDocument) *domain.Notificacao {
	return &domain.Notificacao{
		ID:        doc.ID.Hex(),
		UsuarioID: doc.UsuarioID,
		Tipo:      domain.TipoNotificacao(doc.Tipo),
		Status:    domain.StatusNotificacao(doc.Status),
		Titulo:    doc.Titulo,
		Mensagem:  doc.Mensagem,
		Dados:     doc.Dados,
		CriadoEm: doc.CriadoEm,
		LidoEm:   doc.LidoEm,
	}
}

// NotificacaoMongoRepository implements portout.NotificacaoRepository.
type NotificacaoMongoRepository struct {
	col *mongo.Collection
}

// NewNotificacaoRepository creates a new NotificacaoMongoRepository.
func NewNotificacaoRepository(client *Client) *NotificacaoMongoRepository {
	return &NotificacaoMongoRepository{
		col: client.Collection("notificacoes"),
	}
}

func (r *NotificacaoMongoRepository) FindByID(ctx context.Context, id string) (*domain.Notificacao, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apierr.NotFound("notificacao", id)
	}
	var doc notificacaoDocument
	if err := r.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("notificacao", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return notDocToDomain(doc), nil
}

func (r *NotificacaoMongoRepository) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Notificacao, int64, error) {
	filter := bson.M{"usuario_id": usuarioID}
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

	var docs []notificacaoDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}

	result := make([]domain.Notificacao, len(docs))
	for i, d := range docs {
		result[i] = *notDocToDomain(d)
	}
	return result, total, nil
}

func (r *NotificacaoMongoRepository) FindUnreadByUsuarioID(ctx context.Context, usuarioID string) ([]domain.Notificacao, error) {
	filter := bson.M{
		"usuario_id": usuarioID,
		"status":     string(domain.StatusNotificacaoNaoLida),
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)

	var docs []notificacaoDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, apierr.Internal(err.Error())
	}

	result := make([]domain.Notificacao, len(docs))
	for i, d := range docs {
		result[i] = *notDocToDomain(d)
	}
	return result, nil
}

func (r *NotificacaoMongoRepository) Save(ctx context.Context, n *domain.Notificacao) (*domain.Notificacao, error) {
	doc := notificacaoDocument{
		ID:        bson.NewObjectID(),
		UsuarioID: n.UsuarioID,
		Tipo:      string(n.Tipo),
		Status:    string(n.Status),
		Titulo:    n.Titulo,
		Mensagem:  n.Mensagem,
		Dados:     n.Dados,
		CriadoEm: n.CriadoEm,
		LidoEm:   n.LidoEm,
	}
	_, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	n.ID = doc.ID.Hex()
	return n, nil
}

func (r *NotificacaoMongoRepository) Update(ctx context.Context, n *domain.Notificacao) (*domain.Notificacao, error) {
	oid, err := bson.ObjectIDFromHex(n.ID)
	if err != nil {
		return nil, apierr.NotFound("notificacao", n.ID)
	}
	doc := notificacaoDocument{
		ID:        oid,
		UsuarioID: n.UsuarioID,
		Tipo:      string(n.Tipo),
		Status:    string(n.Status),
		Titulo:    n.Titulo,
		Mensagem:  n.Mensagem,
		Dados:     n.Dados,
		CriadoEm: n.CriadoEm,
		LidoEm:   n.LidoEm,
	}
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": oid}, doc, options.Replace().SetUpsert(false))
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return nil, apierr.NotFound("notificacao", n.ID)
	}
	return n, nil
}

func (r *NotificacaoMongoRepository) MarkAllRead(ctx context.Context, usuarioID string) error {
	now := time.Now()
	_, err := r.col.UpdateMany(ctx,
		bson.M{"usuario_id": usuarioID, "status": string(domain.StatusNotificacaoNaoLida)},
		bson.M{"$set": bson.M{
			"status":  string(domain.StatusNotificacaoLida),
			"lido_em": now,
		}},
	)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

func (r *NotificacaoMongoRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.NotFound("notificacao", id)
	}
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("notificacao", id)
	}
	return nil
}
