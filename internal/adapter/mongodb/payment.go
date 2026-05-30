package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- PagamentoMensal ----

type pagamentoDocument struct {
	ID              bson.ObjectID  `bson:"_id,omitempty"`
	EventoID        string         `bson:"evento_id"`
	UsuarioID       string         `bson:"usuario_id"`
	Descricao       string         `bson:"descricao,omitempty"`
	Valor           float64        `bson:"valor"`
	MetodoPagamento string         `bson:"metodo_pagamento"`
	Status          string         `bson:"status"`
	DataVencimento  time.Time      `bson:"data_vencimento"`
	DataPagamento   *time.Time     `bson:"data_pagamento,omitempty"`
	Observacao      string         `bson:"observacao,omitempty"`
	Comprovante     string         `bson:"comprovante,omitempty"`
	CriadoEm       time.Time      `bson:"criado_em"`
	UpdatedAt       time.Time      `bson:"updated_at"`
}

func pagDocFromDomain(p *domain.PagamentoMensal) pagamentoDocument {
	return pagamentoDocument{
		EventoID:        p.EventoID,
		UsuarioID:       p.UsuarioID,
		Descricao:       p.Descricao,
		Valor:           p.Valor,
		MetodoPagamento: string(p.MetodoPagamento),
		Status:          string(p.Status),
		DataVencimento:  p.DataVencimento,
		DataPagamento:   p.DataPagamento,
		Observacao:      p.Observacao,
		Comprovante:     p.Comprovante,
		CriadoEm:       p.CriadoEm,
		UpdatedAt:       p.UpdatedAt,
	}
}

func pagDocToDomain(doc pagamentoDocument) *domain.PagamentoMensal {
	return &domain.PagamentoMensal{
		ID:              doc.ID.Hex(),
		EventoID:        doc.EventoID,
		UsuarioID:       doc.UsuarioID,
		Descricao:       doc.Descricao,
		Valor:           doc.Valor,
		MetodoPagamento: domain.MetodoPagamento(doc.MetodoPagamento),
		Status:          domain.StatusPagamento(doc.Status),
		DataVencimento:  doc.DataVencimento,
		DataPagamento:   doc.DataPagamento,
		Observacao:      doc.Observacao,
		Comprovante:     doc.Comprovante,
		CriadoEm:       doc.CriadoEm,
		UpdatedAt:       doc.UpdatedAt,
	}
}

// PagamentoMongoRepository implements portout.PagamentoMensalRepository.
type PagamentoMongoRepository struct {
	col *mongo.Collection
}

// NewPagamentoRepository creates a new PagamentoMongoRepository.
func NewPagamentoRepository(client *Client) *PagamentoMongoRepository {
	return &PagamentoMongoRepository{col: client.Collection("pagamentos_mensais")}
}

func (r *PagamentoMongoRepository) FindByID(ctx context.Context, id string) (*domain.PagamentoMensal, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apierr.NotFound("pagamento", id)
	}
	var doc pagamentoDocument
	if err := r.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("pagamento", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return pagDocToDomain(doc), nil
}

func (r *PagamentoMongoRepository) FindByEventoID(ctx context.Context, eventoID string) ([]domain.PagamentoMensal, error) {
	cur, err := r.col.Find(ctx, bson.M{"evento_id": eventoID})
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var result []domain.PagamentoMensal
	for cur.Next(ctx) {
		var doc pagamentoDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, *pagDocToDomain(doc))
	}
	return result, nil
}

func (r *PagamentoMongoRepository) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.PagamentoMensal, int64, error) {
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
	var result []domain.PagamentoMensal
	for cur.Next(ctx) {
		var doc pagamentoDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, apierr.Internal(err.Error())
		}
		result = append(result, *pagDocToDomain(doc))
	}
	return result, total, nil
}

func (r *PagamentoMongoRepository) FindByEventoIDAndStatus(ctx context.Context, eventoID string, status domain.StatusPagamento) ([]domain.PagamentoMensal, error) {
	cur, err := r.col.Find(ctx, bson.M{"evento_id": eventoID, "status": string(status)})
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var result []domain.PagamentoMensal
	for cur.Next(ctx) {
		var doc pagamentoDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, *pagDocToDomain(doc))
	}
	return result, nil
}

func (r *PagamentoMongoRepository) Save(ctx context.Context, p *domain.PagamentoMensal) (*domain.PagamentoMensal, error) {
	doc := pagDocFromDomain(p)
	res, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	p.ID = res.InsertedID.(bson.ObjectID).Hex()
	return p, nil
}

func (r *PagamentoMongoRepository) Update(ctx context.Context, p *domain.PagamentoMensal) (*domain.PagamentoMensal, error) {
	oid, err := bson.ObjectIDFromHex(p.ID)
	if err != nil {
		return nil, apierr.NotFound("pagamento", p.ID)
	}
	doc := pagDocFromDomain(p)
	doc.ID = oid
	_, err = r.col.ReplaceOne(ctx, bson.M{"_id": oid}, doc, options.Replace().SetUpsert(false))
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return p, nil
}

func (r *PagamentoMongoRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apierr.NotFound("pagamento", id)
	}
	_, err = r.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// ---- EventoConfigPagamento ----

type configPagamentoDocument struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	EventoID         string        `bson:"evento_id"`
	UsuarioID        string        `bson:"usuario_id"`
	MetodosPagamento []string      `bson:"metodos_pagamento"`
	PrazoPagamento   *time.Time    `bson:"prazo_pagamento,omitempty"`
	ChavePix         string        `bson:"chave_pix,omitempty"`
	InstrucoesBoleto string        `bson:"instrucoes_boleto,omitempty"`
	CriadoEm        time.Time     `bson:"criado_em"`
	UpdatedAt        time.Time     `bson:"updated_at"`
}

func cfgDocToDomain(doc configPagamentoDocument) *domain.EventoConfigPagamento {
	methods := make([]domain.MetodoPagamento, len(doc.MetodosPagamento))
	for i, m := range doc.MetodosPagamento {
		methods[i] = domain.MetodoPagamento(m)
	}
	return &domain.EventoConfigPagamento{
		ID:               doc.ID.Hex(),
		EventoID:         doc.EventoID,
		UsuarioID:        doc.UsuarioID,
		MetodosPagamento: methods,
		PrazoPagamento:   doc.PrazoPagamento,
		ChavePix:         doc.ChavePix,
		InstrucoesBoleto: doc.InstrucoesBoleto,
		CriadoEm:        doc.CriadoEm,
		UpdatedAt:        doc.UpdatedAt,
	}
}

// ConfigPagamentoMongoRepository implements portout.EventoConfigPagamentoRepository.
type ConfigPagamentoMongoRepository struct {
	col *mongo.Collection
}

// NewConfigPagamentoRepository creates a new ConfigPagamentoMongoRepository.
func NewConfigPagamentoRepository(client *Client) *ConfigPagamentoMongoRepository {
	return &ConfigPagamentoMongoRepository{col: client.Collection("evento_config_pagamentos")}
}

func (r *ConfigPagamentoMongoRepository) FindByEventoID(ctx context.Context, eventoID string) (*domain.EventoConfigPagamento, error) {
	var doc configPagamentoDocument
	err := r.col.FindOne(ctx, bson.M{"evento_id": eventoID}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, apierr.NotFound("config_pagamento", eventoID)
	}
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return cfgDocToDomain(doc), nil
}

func (r *ConfigPagamentoMongoRepository) Save(ctx context.Context, cfg *domain.EventoConfigPagamento) (*domain.EventoConfigPagamento, error) {
	methods := make([]string, len(cfg.MetodosPagamento))
	for i, m := range cfg.MetodosPagamento {
		methods[i] = string(m)
	}
	doc := configPagamentoDocument{
		EventoID:         cfg.EventoID,
		UsuarioID:        cfg.UsuarioID,
		MetodosPagamento: methods,
		PrazoPagamento:   cfg.PrazoPagamento,
		ChavePix:         cfg.ChavePix,
		InstrucoesBoleto: cfg.InstrucoesBoleto,
		CriadoEm:        cfg.CriadoEm,
		UpdatedAt:        cfg.UpdatedAt,
	}
	res, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	cfg.ID = res.InsertedID.(bson.ObjectID).Hex()
	return cfg, nil
}

func (r *ConfigPagamentoMongoRepository) Update(ctx context.Context, cfg *domain.EventoConfigPagamento) (*domain.EventoConfigPagamento, error) {
	oid, err := bson.ObjectIDFromHex(cfg.ID)
	if err != nil {
		return nil, apierr.NotFound("config_pagamento", cfg.ID)
	}
	methods := make([]string, len(cfg.MetodosPagamento))
	for i, m := range cfg.MetodosPagamento {
		methods[i] = string(m)
	}
	doc := configPagamentoDocument{
		ID:               oid,
		EventoID:         cfg.EventoID,
		UsuarioID:        cfg.UsuarioID,
		MetodosPagamento: methods,
		PrazoPagamento:   cfg.PrazoPagamento,
		ChavePix:         cfg.ChavePix,
		InstrucoesBoleto: cfg.InstrucoesBoleto,
		CriadoEm:        cfg.CriadoEm,
		UpdatedAt:        cfg.UpdatedAt,
	}
	_, err = r.col.ReplaceOne(ctx, bson.M{"_id": oid}, doc, options.Replace().SetUpsert(false))
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return cfg, nil
}
