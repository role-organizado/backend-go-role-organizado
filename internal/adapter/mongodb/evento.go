package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

const colEventos = "eventos"

type eventoDocument struct {
	ID                   bson.ObjectID `bson:"_id,omitempty"`
	UsuarioID            string        `bson:"usuario_id"`
	Nome                 string        `bson:"nome"`
	Tipo                 string        `bson:"tipo"`
	Data                 time.Time     `bson:"data"`
	Descricao            string        `bson:"descricao"`
	Local                string        `bson:"local"`
	FotoURL              string        `bson:"foto_url"`
	Status               string        `bson:"status"`
	ConvidadosIDs        []string      `bson:"convidados_ids"`
	PoliticaConvidados   string        `bson:"politica_convidados"`
	LimiteConvidados     *int          `bson:"limite_convidados"`
	RateiosHabilitado    bool          `bson:"rateios_habilitado"`
	TipoDivisaoRateio    string        `bson:"tipo_divisao_rateio"`
	PagamentosHabilitado bool          `bson:"pagamentos_habilitado"`
	MetodosPagamento     []string      `bson:"metodos_pagamento"`
	PrazoPagamento       *time.Time    `bson:"prazo_pagamento"`
	RegrasCustomizadas   string        `bson:"regras_customizadas"`
	PoliticaCancelamento string        `bson:"politica_cancelamento"`
	CriadoEm            time.Time     `bson:"criado_em"`
	UpdatedAt            time.Time     `bson:"updated_at"`
}

// EventoRepository implements portout.EventoRepository using MongoDB.
type EventoRepository struct {
	col *mongo.Collection
}

// NewEventoRepository creates a new EventoRepository.
func NewEventoRepository(client *Client) *EventoRepository {
	return &EventoRepository{col: client.Collection(colEventos)}
}

// FindByID retrieves an event by its MongoDB ObjectID.
func (r *EventoRepository) FindByID(ctx context.Context, id string) (*domain.Evento, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apierr.NotFound("evento", id)
	}
	var doc eventoDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("evento", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	e := eventoFromDoc(doc)
	return &e, nil
}

// FindByUsuarioID paginates events belonging to the given user.
func (r *EventoRepository) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Evento, int64, error) {
	filter := bson.D{{Key: "usuario_id", Value: usuarioID}}
	return r.findPaginated(ctx, filter, page, pageSize)
}

// FindAll paginates all events.
func (r *EventoRepository) FindAll(ctx context.Context, page, pageSize int) ([]domain.Evento, int64, error) {
	return r.findPaginated(ctx, bson.D{}, page, pageSize)
}

func (r *EventoRepository) findPaginated(ctx context.Context, filter bson.D, page, pageSize int) ([]domain.Evento, int64, error) {
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	skip := int64((page - 1) * pageSize)
	opts := options.Find().SetSkip(skip).SetLimit(int64(pageSize)).SetSort(bson.D{{Key: "data", Value: -1}})
	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)
	var docs []eventoDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	eventos := make([]domain.Evento, len(docs))
	for i, d := range docs {
		eventos[i] = eventoFromDoc(d)
	}
	return eventos, total, nil
}

// Save inserts a new event and returns it with the generated ID.
func (r *EventoRepository) Save(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	doc := eventoToDoc(e)
	doc.ID = bson.NewObjectID()
	_, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	saved := eventoFromDoc(doc)
	return &saved, nil
}

// Update replaces an existing event document.
func (r *EventoRepository) Update(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	oid, err := bson.ObjectIDFromHex(e.ID)
	if err != nil {
		return nil, apierr.NotFound("evento", e.ID)
	}
	doc := eventoToDoc(e)
	doc.ID = oid
	filter := bson.D{{Key: "_id", Value: oid}}
	_, err = r.col.ReplaceOne(ctx, filter, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return e, nil
}

// DeleteByID removes an event by its ObjectID.
func (r *EventoRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.NotFound("evento", id)
	}
	res, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("evento", id)
	}
	return nil
}

// ---- helpers ----

func eventoFromDoc(doc eventoDocument) domain.Evento {
	return domain.Evento{
		ID:                   doc.ID.Hex(),
		UsuarioID:            doc.UsuarioID,
		Nome:                 doc.Nome,
		Tipo:                 doc.Tipo,
		Data:                 doc.Data,
		Descricao:            doc.Descricao,
		Local:                doc.Local,
		FotoURL:              doc.FotoURL,
		Status:               domain.EventoStatus(doc.Status),
		ConvidadosIDs:        doc.ConvidadosIDs,
		PoliticaConvidados:   doc.PoliticaConvidados,
		LimiteConvidados:     doc.LimiteConvidados,
		RateiosHabilitado:    doc.RateiosHabilitado,
		TipoDivisaoRateio:    doc.TipoDivisaoRateio,
		PagamentosHabilitado: doc.PagamentosHabilitado,
		MetodosPagamento:     doc.MetodosPagamento,
		PrazoPagamento:       doc.PrazoPagamento,
		RegrasCustomizadas:   doc.RegrasCustomizadas,
		PoliticaCancelamento: doc.PoliticaCancelamento,
		CriadoEm:             doc.CriadoEm,
		UpdatedAt:            doc.UpdatedAt,
	}
}

func eventoToDoc(e *domain.Evento) eventoDocument {
	return eventoDocument{
		UsuarioID:            e.UsuarioID,
		Nome:                 e.Nome,
		Tipo:                 e.Tipo,
		Data:                 e.Data,
		Descricao:            e.Descricao,
		Local:                e.Local,
		FotoURL:              e.FotoURL,
		Status:               string(e.Status),
		ConvidadosIDs:        e.ConvidadosIDs,
		PoliticaConvidados:   e.PoliticaConvidados,
		LimiteConvidados:     e.LimiteConvidados,
		RateiosHabilitado:    e.RateiosHabilitado,
		TipoDivisaoRateio:    e.TipoDivisaoRateio,
		PagamentosHabilitado: e.PagamentosHabilitado,
		MetodosPagamento:     e.MetodosPagamento,
		PrazoPagamento:       e.PrazoPagamento,
		RegrasCustomizadas:   e.RegrasCustomizadas,
		PoliticaCancelamento: e.PoliticaCancelamento,
		CriadoEm:             e.CriadoEm,
		UpdatedAt:            e.UpdatedAt,
	}
}
