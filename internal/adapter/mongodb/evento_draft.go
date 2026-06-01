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
	UsuarioID string        `bson:"usuarioId"` // camelCase: matches Java MongoDB schema

	// Etapa 0
	Nome      string     `bson:"nome"`
	Tipo      string     `bson:"tipo"`
	Data      *time.Time `bson:"data"`
	Descricao string     `bson:"descricao"`
	Local     string     `bson:"local"`

	// Etapa 1
	ConvidadosIDs      []string `bson:"convidadosIds"` // camelCase: matches Java
	PoliticaConvidados string   `bson:"politicaConvidados"`
	LimiteConvidados   *int     `bson:"limiteConvidados"`

	// Etapa 2
	RateiosHabilitado  bool                 `bson:"rateiosHabilitado"`
	RateiosItens       []rateioItemDocument `bson:"rateiosItens"`
	TipoDivisaoRateio  string               `bson:"tipoDivisaoRateio"`

	// Etapa 3
	PagamentosHabilitado bool       `bson:"pagamentosHabilitado"`
	MetodosPagamento     []string   `bson:"metodosPagamento"`
	PrazoPagamento       *time.Time `bson:"prazoPagamento"`

	// Etapa 4
	RegrasCustomizadas   []string `bson:"regrasCustomizadas"`
	PoliticaCancelamento string   `bson:"politicaCancelamento"`

	// Nicho modules
	ModulosAtivos     []string       `bson:"modulos_ativos,omitempty"`
	ConfiguracaoNicho map[string]any `bson:"configuracao_nicho,omitempty"`

	// Wizard state
	EtapaAtual      int   `bson:"etapaAtual"` // camelCase: matches Java MongoDB schema
	EtapasCompletas []int `bson:"etapasCompletas"`

	CriadoEm    time.Time `bson:"criadoEm"`    // camelCase: required by schema validator
	AtualizadoEm time.Time `bson:"atualizadoEm"` // camelCase: required + TTL index
}

// EventoDraftRepository implements portout.EventoDraftRepository using MongoDB.
// The `eventos_draft` collection has a TTL index on `atualizadoEm` (7776000s = 90 days).
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
	filter := bson.D{{Key: "usuarioId", Value: usuarioID}}
	opts := options.Find().SetSort(bson.D{{Key: "atualizadoEm", Value: -1}})
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
	now := time.Now().UTC()
	if doc.CriadoEm.IsZero() {
		doc.CriadoEm = now
	}
	doc.AtualizadoEm = now
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
	doc.AtualizadoEm = time.Now().UTC()
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
	// Convert []string regrasCustomizadas back to string for domain model
	var regrasStr string
	if len(doc.RegrasCustomizadas) > 0 {
		regrasStr = doc.RegrasCustomizadas[0]
	}
	// Ensure non-nil slices
	convidadosIDs := doc.ConvidadosIDs
	if convidadosIDs == nil {
		convidadosIDs = []string{}
	}
	metodosPagamento := doc.MetodosPagamento
	if metodosPagamento == nil {
		metodosPagamento = []string{}
	}
	etapasCompletas := doc.EtapasCompletas
	if etapasCompletas == nil {
		etapasCompletas = []int{}
	}
	modulosAtivos := doc.ModulosAtivos
	if modulosAtivos == nil {
		modulosAtivos = []string{}
	}
	return domain.EventoDraft{
		ID:                   doc.ID.Hex(),
		UsuarioID:            doc.UsuarioID,
		Nome:                 doc.Nome,
		Tipo:                 doc.Tipo,
		Data:                 doc.Data,
		Descricao:            doc.Descricao,
		Local:                doc.Local,
		ConvidadosIDs:        convidadosIDs,
		PoliticaConvidados:   doc.PoliticaConvidados,
		LimiteConvidados:     doc.LimiteConvidados,
		RateiosHabilitado:    doc.RateiosHabilitado,
		RateiosItens:         itens,
		TipoDivisaoRateio:    doc.TipoDivisaoRateio,
		PagamentosHabilitado: doc.PagamentosHabilitado,
		MetodosPagamento:     metodosPagamento,
		PrazoPagamento:       doc.PrazoPagamento,
		RegrasCustomizadas:   regrasStr,
		PoliticaCancelamento: doc.PoliticaCancelamento,
		EtapaAtual:           doc.EtapaAtual,
		EtapasCompletas:      etapasCompletas,
		ModulosAtivos:        modulosAtivos,
		ConfiguracaoNicho:    doc.ConfiguracaoNicho,
		CriadoEm:             doc.CriadoEm,
		UpdatedAt:            doc.AtualizadoEm,
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
	// Ensure all slice fields are non-nil to satisfy MongoDB array schema
	convidadosIDs := d.ConvidadosIDs
	if convidadosIDs == nil {
		convidadosIDs = []string{}
	}
	metodosPagamento := d.MetodosPagamento
	if metodosPagamento == nil {
		metodosPagamento = []string{}
	}
	etapasCompletas := d.EtapasCompletas
	if etapasCompletas == nil {
		etapasCompletas = []int{}
	}
	modulosAtivos := d.ModulosAtivos
	if modulosAtivos == nil {
		modulosAtivos = []string{}
	}
	// regrasCustomizadas is stored as []string in MongoDB (Java compat) but domain uses string
	var regrasCustomizadas []string
	if d.RegrasCustomizadas != "" {
		regrasCustomizadas = []string{d.RegrasCustomizadas}
	} else {
		regrasCustomizadas = []string{}
	}
	return eventoDraftDocument{
		UsuarioID:            d.UsuarioID,
		Nome:                 d.Nome,
		Tipo:                 d.Tipo,
		Data:                 d.Data,
		Descricao:            d.Descricao,
		Local:                d.Local,
		ConvidadosIDs:        convidadosIDs,
		PoliticaConvidados:   d.PoliticaConvidados,
		LimiteConvidados:     d.LimiteConvidados,
		RateiosHabilitado:    d.RateiosHabilitado,
		RateiosItens:         itens,
		TipoDivisaoRateio:    d.TipoDivisaoRateio,
		PagamentosHabilitado: d.PagamentosHabilitado,
		MetodosPagamento:     metodosPagamento,
		PrazoPagamento:       d.PrazoPagamento,
		RegrasCustomizadas:   regrasCustomizadas,
		PoliticaCancelamento: d.PoliticaCancelamento,
		EtapaAtual:           d.EtapaAtual,
		EtapasCompletas:      etapasCompletas,
		ModulosAtivos:        modulosAtivos,
		ConfiguracaoNicho:    d.ConfiguracaoNicho,
		CriadoEm:             d.CriadoEm,
		AtualizadoEm:         d.UpdatedAt,
	}
}
