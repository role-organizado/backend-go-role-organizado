package mongodb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/cofrinho"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

const colCofrinhoContribuicoes = "cofrinho_contribuicoes"

// cofrinhoDocument matches the MongoDB schema for cofrinho_contribuicoes.
type cofrinhoDocument struct {
	ID               string    `bson:"_id,omitempty"`
	EventoID         string    `bson:"evento_id"`
	GuestID          string    `bson:"guest_id"`
	Nome             string    `bson:"nome"`
	Mensagem         string    `bson:"mensagem,omitempty"`
	Valor            int64     `bson:"valor"`
	Status           string    `bson:"status"`
	PIXQRCode        string    `bson:"pix_qr_code,omitempty"`
	WebhookPaymentID string    `bson:"webhook_payment_id,omitempty"`
	CriadoEm        time.Time `bson:"criado_em"`
	AtualizadoEm    time.Time `bson:"atualizado_em"`
}

// CofrinhoRepository implements portout.CofrinhoRepository using MongoDB.
type CofrinhoRepository struct {
	col *mongo.Collection
}

// NewCofrinhoRepository creates a new CofrinhoRepository.
func NewCofrinhoRepository(client *Client) *CofrinhoRepository {
	return &CofrinhoRepository{col: client.Collection(colCofrinhoContribuicoes)}
}

// Save persists a new cofrinho contribution.
func (r *CofrinhoRepository) Save(ctx context.Context, c *domain.CofrinhoContribuicao) (*domain.CofrinhoContribuicao, error) {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	doc := cofrinhoToDoc(c)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return c, nil
}

// FindByID retrieves a contribution by its string ID.
func (r *CofrinhoRepository) FindByID(ctx context.Context, id string) (*domain.CofrinhoContribuicao, error) {
	var doc cofrinhoDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("cofrinho_contribuicao", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	c := cofrinhoFromDoc(doc)
	return &c, nil
}

// FindByEventoID retrieves all contributions for a given event.
func (r *CofrinhoRepository) FindByEventoID(ctx context.Context, eventoID string) ([]*domain.CofrinhoContribuicao, error) {
	cursor, err := r.col.Find(ctx, bson.D{{Key: "evento_id", Value: eventoID}},
		options.Find().SetSort(bson.D{{Key: "criado_em", Value: -1}}),
	)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)

	var results []*domain.CofrinhoContribuicao
	for cursor.Next(ctx) {
		var doc cofrinhoDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		c := cofrinhoFromDoc(doc)
		results = append(results, &c)
	}
	if err := cursor.Err(); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if results == nil {
		results = []*domain.CofrinhoContribuicao{}
	}
	return results, nil
}

// UpdateStatus updates the status and optional webhook payment ID of a contribution.
func (r *CofrinhoRepository) UpdateStatus(ctx context.Context, id string, status domain.CofrinhoStatus, webhookPaymentID string) (*domain.CofrinhoContribuicao, error) {
	now := time.Now()
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status", Value: string(status)},
			{Key: "webhook_payment_id", Value: webhookPaymentID},
			{Key: "atualizado_em", Value: now},
		}},
	}
	var doc cofrinhoDocument
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	if err := r.col.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: id}}, update, opts).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("cofrinho_contribuicao", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	c := cofrinhoFromDoc(doc)
	return &c, nil
}

func cofrinhoToDoc(c *domain.CofrinhoContribuicao) cofrinhoDocument {
	return cofrinhoDocument{
		ID:               c.ID,
		EventoID:         c.EventoID,
		GuestID:          c.GuestID,
		Nome:             c.Nome,
		Mensagem:         c.Mensagem,
		Valor:            c.Valor,
		Status:           string(c.Status),
		PIXQRCode:        c.PIXQRCode,
		WebhookPaymentID: c.WebhookPaymentID,
		CriadoEm:        c.CriadoEm,
		AtualizadoEm:    c.UpdatedAt,
	}
}

func cofrinhoFromDoc(doc cofrinhoDocument) domain.CofrinhoContribuicao {
	return domain.CofrinhoContribuicao{
		ID:               doc.ID,
		EventoID:         doc.EventoID,
		GuestID:          doc.GuestID,
		Nome:             doc.Nome,
		Mensagem:         doc.Mensagem,
		Valor:            doc.Valor,
		Status:           domain.CofrinhoStatus(doc.Status),
		PIXQRCode:        doc.PIXQRCode,
		WebhookPaymentID: doc.WebhookPaymentID,
		CriadoEm:        doc.CriadoEm,
		UpdatedAt:        doc.AtualizadoEm,
	}
}
