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

const colEventosDraft = "eventos_draft"

type rateioItemDocument struct {
	Descricao  string  `bson:"descricao"`
	Valor      float64 `bson:"valor"`
	Quantidade int     `bson:"quantidade"`
}

type eventoDraftDocument struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	UsuarioID string        `bson:"usuario_id"`

	// Etapa 0
	Nome      string     `bson:"nome"`
	Tipo      string     `bson:"tipo"`
	Data      *time.Time `bson:"data"`
	Descricao string     `bson:"descricao"`
	Local     string     `bson:"local"`

	// Etapa 1
	ConvidadosIDs      []string `bson:"convidados_ids"`
	PoliticaConvidados string   `bson:"politica_convidados"`
	LimiteConvidados   *int     `bson:"limite_convidados"`

	// Etapa 2
	RateiosHabilitado  bool                 `bson:"rateios_habilitado"`
	RateiosItens       []rateioItemDocument `bson:"rateios_itens"`
	TipoDivisaoRateio  string               `bson:"tipo_divisao_rateio"`

	// Etapa 3
	PagamentosHabilitado bool       `bson:"pagamentos_habilitado"`
	MetodosPagamento     []string   `bson:"metodos_pagamento"`
	PrazoPagamento       *time.Time `bson:"prazo_pagamento"`

	// Etapa 4
	RegrasCustomizadas   string `bson:"regras_customizadas"`
	PoliticaCancelamento string `bson:"politica_cancelamento"`

	// Wizard state
	EtapaAtual      int   `bson:"etapa_atual"`
	EtapasCompletas []int `bson:"etapas_completas"`

	CriadoEm  time.Time `bson:"criado_em"`
	UpdatedAt time.Time `bson:"updated_at"` // TTL index on this field (90 days)
}

// EventoDraftRepository implements portout.EventoDraftRepository using MongoDB.
// The `eventos_draft` collection has a TTL index on `updated_at` (7776000s = 90 days).
type EventoDraftRepository struct {
	col *mongo.Collection
}

// NewEventoDraftRepository creates a new EventoDraftRepository.
func NewEventoDraftRepository(client *Client) *EventoDraftRepository {
	return &EventoDraftRepository{col: client.Collection(colEventosDraft)}
}

// FindByID retrieves a draft by its ObjectID.
func (r *EventoDraftRepository) FindByID(ctx context.Context, id string) (*domain.EventoDraft, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apierr.NotFound("draft", id)
	}
	var doc eventoDraftDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "_id", Value: oid}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("draft", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	d := draftFromDoc(doc)
	return &d, nil
}

// FindByUsuarioID returns all drafts for the given user, sorted by last update.
func (r *EventoDraftRepository) FindByUsuarioID(ctx context.Context, usuarioID string) ([]domain.EventoDraft, error) {
	filter := bson.D{{Key: "usuario_id", Value: usuarioID}}
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)
	var docs []eventoDraftDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	drafts := make([]domain.EventoDraft, len(docs))
	for i, d := range docs {
		drafts[i] = draftFromDoc(d)
	}
	return drafts, nil
}

// Save inserts a new draft and returns it with the generated ID.
func (r *EventoDraftRepository) Save(ctx context.Context, d *domain.EventoDraft) (*domain.EventoDraft, error) {
	doc := draftToDoc(d)
	doc.ID = bson.NewObjectID()
	_, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	saved := draftFromDoc(doc)
	return &saved, nil
}

// Update replaces an existing draft document.
func (r *EventoDraftRepository) Update(ctx context.Context, d *domain.EventoDraft) (*domain.EventoDraft, error) {
	oid, err := bson.ObjectIDFromHex(d.ID)
	if err != nil {
		return nil, apierr.NotFound("draft", d.ID)
	}
	doc := draftToDoc(d)
	doc.ID = oid
	filter := bson.D{{Key: "_id", Value: oid}}
	_, err = r.col.ReplaceOne(ctx, filter, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return d, nil
}

// DeleteByID removes a draft by its ObjectID.
func (r *EventoDraftRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.NotFound("draft", id)
	}
	res, err := r.col.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("draft", id)
	}
	return nil
}

// ---- helpers ----

func draftFromDoc(doc eventoDraftDocument) domain.EventoDraft {
	itens := make([]domain.RateioItem, len(doc.RateiosItens))
	for i, ri := range doc.RateiosItens {
		itens[i] = domain.RateioItem{
			Descricao:  ri.Descricao,
			Valor:      ri.Valor,
			Quantidade: ri.Quantidade,
		}
	}
	return domain.EventoDraft{
		ID:                   doc.ID.Hex(),
		UsuarioID:            doc.UsuarioID,
		Nome:                 doc.Nome,
		Tipo:                 doc.Tipo,
		Data:                 doc.Data,
		Descricao:            doc.Descricao,
		Local:                doc.Local,
		ConvidadosIDs:        doc.ConvidadosIDs,
		PoliticaConvidados:   doc.PoliticaConvidados,
		LimiteConvidados:     doc.LimiteConvidados,
		RateiosHabilitado:    doc.RateiosHabilitado,
		RateiosItens:         itens,
		TipoDivisaoRateio:    doc.TipoDivisaoRateio,
		PagamentosHabilitado: doc.PagamentosHabilitado,
		MetodosPagamento:     doc.MetodosPagamento,
		PrazoPagamento:       doc.PrazoPagamento,
		RegrasCustomizadas:   doc.RegrasCustomizadas,
		PoliticaCancelamento: doc.PoliticaCancelamento,
		EtapaAtual:           doc.EtapaAtual,
		EtapasCompletas:      doc.EtapasCompletas,
		CriadoEm:             doc.CriadoEm,
		UpdatedAt:            doc.UpdatedAt,
	}
}

func draftToDoc(d *domain.EventoDraft) eventoDraftDocument {
	itens := make([]rateioItemDocument, len(d.RateiosItens))
	for i, ri := range d.RateiosItens {
		itens[i] = rateioItemDocument{
			Descricao:  ri.Descricao,
			Valor:      ri.Valor,
			Quantidade: ri.Quantidade,
		}
	}
	return eventoDraftDocument{
		UsuarioID:            d.UsuarioID,
		Nome:                 d.Nome,
		Tipo:                 d.Tipo,
		Data:                 d.Data,
		Descricao:            d.Descricao,
		Local:                d.Local,
		ConvidadosIDs:        d.ConvidadosIDs,
		PoliticaConvidados:   d.PoliticaConvidados,
		LimiteConvidados:     d.LimiteConvidados,
		RateiosHabilitado:    d.RateiosHabilitado,
		RateiosItens:         itens,
		TipoDivisaoRateio:    d.TipoDivisaoRateio,
		PagamentosHabilitado: d.PagamentosHabilitado,
		MetodosPagamento:     d.MetodosPagamento,
		PrazoPagamento:       d.PrazoPagamento,
		RegrasCustomizadas:   d.RegrasCustomizadas,
		PoliticaCancelamento: d.PoliticaCancelamento,
		EtapaAtual:           d.EtapaAtual,
		EtapasCompletas:      d.EtapasCompletas,
		CriadoEm:             d.CriadoEm,
		UpdatedAt:            d.UpdatedAt,
	}
}
