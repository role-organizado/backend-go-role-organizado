package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- rateioDocument ----

type rateioDocument struct {
	ID                  bson.ObjectID        `bson:"_id,omitempty"`
	EventoID            string               `bson:"evento_id"`
	UsuarioID           string               `bson:"usuario_id"`
	Tipo                string               `bson:"tipo"`
	Status              string               `bson:"status"`
	Descricao           string               `bson:"descricao,omitempty"`
	ValorTotal          float64              `bson:"valor_total"`
	NumeroParticipantes int                  `bson:"numero_participantes"`
	Itens               []rateioItemEmbed    `bson:"itens,omitempty"`
	CriadoEm            time.Time            `bson:"criado_em"`
	UpdatedAt           time.Time            `bson:"updated_at"`
}

type rateioItemEmbed struct {
	ID         string    `bson:"id,omitempty"`
	Descricao  string    `bson:"descricao"`
	Valor      float64   `bson:"valor"`
	Quantidade int       `bson:"quantidade"`
	Total      float64   `bson:"total"`
	CriadoEm  time.Time `bson:"criado_em"`
	UpdatedAt  time.Time `bson:"updated_at"`
}

func rateioDocFromDomain(r *domain.Rateio) rateioDocument {
	itens := make([]rateioItemEmbed, len(r.Itens))
	for i, it := range r.Itens {
		itens[i] = rateioItemEmbed{
			ID:         it.ID,
			Descricao:  it.Descricao,
			Valor:      it.Valor,
			Quantidade: it.Quantidade,
			Total:      it.Total,
			CriadoEm:  it.CriadoEm,
			UpdatedAt:  it.UpdatedAt,
		}
	}
	return rateioDocument{
		EventoID:            r.EventoID,
		UsuarioID:           r.UsuarioID,
		Tipo:                string(r.Tipo),
		Status:              string(r.Status),
		Descricao:           r.Descricao,
		ValorTotal:          r.ValorTotal,
		NumeroParticipantes: r.NumeroParticipantes,
		Itens:               itens,
		CriadoEm:            r.CriadoEm,
		UpdatedAt:           r.UpdatedAt,
	}
}

func ratiodocToDomain(doc rateioDocument) *domain.Rateio {
	itens := make([]domain.RateioItem, len(doc.Itens))
	for i, it := range doc.Itens {
		itens[i] = domain.RateioItem{
			ID:         it.ID,
			RateioID:   doc.ID.Hex(),
			Descricao:  it.Descricao,
			Valor:      it.Valor,
			Quantidade: it.Quantidade,
			Total:      it.Total,
			CriadoEm:  it.CriadoEm,
			UpdatedAt:  it.UpdatedAt,
		}
	}
	return &domain.Rateio{
		ID:                  doc.ID.Hex(),
		EventoID:            doc.EventoID,
		UsuarioID:           doc.UsuarioID,
		Tipo:                domain.TipoRateio(doc.Tipo),
		Status:              domain.StatusRateio(doc.Status),
		Descricao:           doc.Descricao,
		ValorTotal:          doc.ValorTotal,
		NumeroParticipantes: doc.NumeroParticipantes,
		Itens:               itens,
		CriadoEm:            doc.CriadoEm,
		UpdatedAt:           doc.UpdatedAt,
	}
}

// ---- RateioMongoRepository ----

type RateioMongoRepository struct {
	col *mongo.Collection
}

func NewRateioRepository(client *Client) *RateioMongoRepository {
	return &RateioMongoRepository{col: client.Collection("rateios")}
}

func (r *RateioMongoRepository) FindByID(ctx context.Context, id string) (*domain.Rateio, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apierr.NotFound("rateio", id)
	}
	var doc rateioDocument
	if err := r.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("rateio", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return ratiodocToDomain(doc), nil
}

func (r *RateioMongoRepository) FindByEventoID(ctx context.Context, eventoID string) ([]domain.Rateio, error) {
	cur, err := r.col.Find(ctx, bson.M{"evento_id": eventoID})
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var result []domain.Rateio
	for cur.Next(ctx) {
		var doc rateioDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, *ratiodocToDomain(doc))
	}
	return result, nil
}

func (r *RateioMongoRepository) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Rateio, int64, error) {
	filter := bson.M{"usuario_id": usuarioID}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	skip := int64((page - 1) * pageSize)
	opts := options.Find().SetSkip(skip).SetLimit(int64(pageSize))
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var result []domain.Rateio
	for cur.Next(ctx) {
		var doc rateioDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, apierr.Internal(err.Error())
		}
		result = append(result, *ratiodocToDomain(doc))
	}
	return result, total, nil
}

func (r *RateioMongoRepository) Save(ctx context.Context, rat *domain.Rateio) (*domain.Rateio, error) {
	doc := rateioDocFromDomain(rat)
	res, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	rat.ID = res.InsertedID.(bson.ObjectID).Hex()
	return rat, nil
}

func (r *RateioMongoRepository) Update(ctx context.Context, rat *domain.Rateio) (*domain.Rateio, error) {
	oid, err := bson.ObjectIDFromHex(rat.ID)
	if err != nil {
		return nil, apierr.NotFound("rateio", rat.ID)
	}
	doc := rateioDocFromDomain(rat)
	doc.ID = oid
	_, err = r.col.ReplaceOne(ctx, bson.M{"_id": oid}, doc, options.Replace().SetUpsert(false))
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return rat, nil
}

func (r *RateioMongoRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.NotFound("rateio", id)
	}
	_, err = r.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// ---- rateioFechamentoDocument ----

type rateioFechamentoDocument struct {
	ID            bson.ObjectID             `bson:"_id,omitempty"`
	RateioID      string                    `bson:"rateio_id"`
	EventoID      string                    `bson:"evento_id"`
	Versao        int                       `bson:"versao"`
	ValorTotal    float64                   `bson:"valor_total"`
	Participantes []fechamentoPartDoc       `bson:"participantes,omitempty"`
	CriadoEm      time.Time                 `bson:"criado_em"`
}

type fechamentoPartDoc struct {
	UsuarioID  string     `bson:"usuario_id"`
	Valor      float64    `bson:"valor"`
	Percentual float64    `bson:"percentual"`
	Pago       bool       `bson:"pago"`
	PagoEm    *time.Time `bson:"pago_em,omitempty"`
}

func fechamentoToDomain(doc rateioFechamentoDocument) *domain.RateioFechamento {
	parts := make([]domain.FechamentoParticipante, len(doc.Participantes))
	for i, p := range doc.Participantes {
		parts[i] = domain.FechamentoParticipante{
			UsuarioID:  p.UsuarioID,
			Valor:      p.Valor,
			Percentual: p.Percentual,
			Pago:       p.Pago,
			PagoEm:    p.PagoEm,
		}
	}
	return &domain.RateioFechamento{
		ID:            doc.ID.Hex(),
		RateioID:      doc.RateioID,
		EventoID:      doc.EventoID,
		Versao:        doc.Versao,
		ValorTotal:    doc.ValorTotal,
		Participantes: parts,
		CriadoEm:     doc.CriadoEm,
	}
}

// ---- RateioFechamentoMongoRepository ----

type RateioFechamentoMongoRepository struct {
	col *mongo.Collection
}

func NewRateioFechamentoRepository(client *Client) *RateioFechamentoMongoRepository {
	return &RateioFechamentoMongoRepository{col: client.Collection("rateio_fechamentos")}
}

func (r *RateioFechamentoMongoRepository) FindByRateioID(ctx context.Context, rateioID string) ([]domain.RateioFechamento, error) {
	cur, err := r.col.Find(ctx, bson.M{"rateio_id": rateioID},
		options.Find().SetSort(bson.D{{Key: "versao", Value: 1}}))
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var result []domain.RateioFechamento
	for cur.Next(ctx) {
		var doc rateioFechamentoDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, *fechamentoToDomain(doc))
	}
	return result, nil
}

func (r *RateioFechamentoMongoRepository) FindLatestByRateioID(ctx context.Context, rateioID string) (*domain.RateioFechamento, error) {
	var doc rateioFechamentoDocument
	opts := options.FindOne().SetSort(bson.D{{Key: "versao", Value: -1}})
	err := r.col.FindOne(ctx, bson.M{"rateio_id": rateioID}, opts).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, apierr.NotFound("fechamento", rateioID)
	}
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return fechamentoToDomain(doc), nil
}

func (r *RateioFechamentoMongoRepository) Save(ctx context.Context, f *domain.RateioFechamento) (*domain.RateioFechamento, error) {
	parts := make([]fechamentoPartDoc, len(f.Participantes))
	for i, p := range f.Participantes {
		parts[i] = fechamentoPartDoc{
			UsuarioID:  p.UsuarioID,
			Valor:      p.Valor,
			Percentual: p.Percentual,
			Pago:       p.Pago,
			PagoEm:    p.PagoEm,
		}
	}
	doc := rateioFechamentoDocument{
		RateioID:      f.RateioID,
		EventoID:      f.EventoID,
		Versao:        f.Versao,
		ValorTotal:    f.ValorTotal,
		Participantes: parts,
		CriadoEm:     f.CriadoEm,
	}
	res, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	f.ID = res.InsertedID.(bson.ObjectID).Hex()
	return f, nil
}
