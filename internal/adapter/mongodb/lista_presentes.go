package mongodb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/listapresentes"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

const colListaPresentesItens = "lista_presentes_itens"

// listaPresentesDocument matches the MongoDB schema for lista_presentes_itens.
type listaPresentesDocument struct {
	ID                  string    `bson:"_id,omitempty"`
	EventoID            string    `bson:"evento_id"`
	OwnerUserID         string    `bson:"owner_user_id"`
	Nome                string    `bson:"nome"`
	Descricao           string    `bson:"descricao,omitempty"`
	URLProduto          string    `bson:"url_produto,omitempty"`
	Valor               int64     `bson:"valor"`
	Quantidade          int       `bson:"quantidade"`
	Reservado           int       `bson:"reservado"`
	Status              string    `bson:"status"`
	ReservadoPorGuestID string    `bson:"reservado_por_guest_id,omitempty"`
	CriadoEm           time.Time `bson:"criado_em"`
	AtualizadoEm       time.Time `bson:"atualizado_em"`
}

// ListaPresentesRepository implements portout.ListaPresentesRepository using MongoDB.
type ListaPresentesRepository struct {
	col *mongo.Collection
}

// NewListaPresentesRepository creates a new ListaPresentesRepository.
func NewListaPresentesRepository(client *Client) *ListaPresentesRepository {
	return &ListaPresentesRepository{col: client.Collection(colListaPresentesItens)}
}

// Save persists a new gift list item.
func (r *ListaPresentesRepository) Save(ctx context.Context, item *domain.ListaPresentesItem) (*domain.ListaPresentesItem, error) {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	doc := listaPresentesToDoc(item)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return item, nil
}

// FindByID retrieves a gift list item by its string ID.
func (r *ListaPresentesRepository) FindByID(ctx context.Context, id string) (*domain.ListaPresentesItem, error) {
	var doc listaPresentesDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("lista_presentes_item", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	item := listaPresentesFromDoc(doc)
	return &item, nil
}

// FindByEventoID retrieves all gift list items for the given event, ordered by creation date.
func (r *ListaPresentesRepository) FindByEventoID(ctx context.Context, eventoID string) ([]*domain.ListaPresentesItem, error) {
	cursor, err := r.col.Find(ctx, bson.D{{Key: "evento_id", Value: eventoID}},
		options.Find().SetSort(bson.D{{Key: "criado_em", Value: 1}}),
	)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)

	var results []*domain.ListaPresentesItem
	for cursor.Next(ctx) {
		var doc listaPresentesDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		item := listaPresentesFromDoc(doc)
		results = append(results, &item)
	}
	if err := cursor.Err(); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if results == nil {
		results = []*domain.ListaPresentesItem{}
	}
	return results, nil
}

// UpdateStatus updates the status, reserved count and last reserver of an item.
func (r *ListaPresentesRepository) UpdateStatus(ctx context.Context, id string, status domain.ListaItemStatus, reservado int, guestID string) (*domain.ListaPresentesItem, error) {
	now := time.Now()
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "status", Value: string(status)},
			{Key: "reservado", Value: reservado},
			{Key: "reservado_por_guest_id", Value: guestID},
			{Key: "atualizado_em", Value: now},
		}},
	}
	var doc listaPresentesDocument
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	if err := r.col.FindOneAndUpdate(ctx, bson.D{{Key: "_id", Value: id}}, update, opts).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("lista_presentes_item", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	item := listaPresentesFromDoc(doc)
	return &item, nil
}

// Delete removes a gift list item by ID.
func (r *ListaPresentesRepository) Delete(ctx context.Context, id string) error {
	res, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("lista_presentes_item", id)
	}
	return nil
}

func listaPresentesToDoc(item *domain.ListaPresentesItem) listaPresentesDocument {
	return listaPresentesDocument{
		ID:                  item.ID,
		EventoID:            item.EventoID,
		OwnerUserID:         item.OwnerUserID,
		Nome:                item.Nome,
		Descricao:           item.Descricao,
		URLProduto:          item.URLProduto,
		Valor:               item.Valor,
		Quantidade:          item.Quantidade,
		Reservado:           item.Reservado,
		Status:              string(item.Status),
		ReservadoPorGuestID: item.ReservadoPorGuestID,
		CriadoEm:           item.CriadoEm,
		AtualizadoEm:       item.UpdatedAt,
	}
}

func listaPresentesFromDoc(doc listaPresentesDocument) domain.ListaPresentesItem {
	return domain.ListaPresentesItem{
		ID:                  doc.ID,
		EventoID:            doc.EventoID,
		OwnerUserID:         doc.OwnerUserID,
		Nome:                doc.Nome,
		Descricao:           doc.Descricao,
		URLProduto:          doc.URLProduto,
		Valor:               doc.Valor,
		Quantidade:          doc.Quantidade,
		Reservado:           doc.Reservado,
		Status:              domain.ListaItemStatus(doc.Status),
		ReservadoPorGuestID: doc.ReservadoPorGuestID,
		CriadoEm:           doc.CriadoEm,
		UpdatedAt:           doc.AtualizadoEm,
	}
}
