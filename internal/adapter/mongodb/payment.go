package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
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

// configPagamentoDocument maps the evento_config_pagamentos collection.
//
// Shared with the Java backend: Java writes camelCase keys (eventoId, platformFeePercent)
// while Go writes snake_case keys (evento_id, metodos_pagamento). Fee fields are stored
// as camelCase to match Java exactly. Pointer types on fee fields allow old documents
// that lack those fields to decode without error (nil → zero value in domain).
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

	// Fee policy snapshot fields — camelCase to match Java AtualizarConfigPagamentoUseCase.
	// Pointer types: nil if absent in old documents (backward-compatible decoding).
	PlatformFeePercent    *float64 `bson:"platformFeePercent,omitempty"`
	PspFeePercent         *float64 `bson:"pspFeePercent,omitempty"`
	PlatformFeeFixedCents *int64   `bson:"platformFeeFixedCents,omitempty"`
	PspFeeFixedCents      *int64   `bson:"pspFeeFixedCents,omitempty"`
	FeePolicyVersion      string   `bson:"feePolicyVersion,omitempty"`

	// Payment processing configuration.
	PaymentProvider       string `bson:"paymentProvider,omitempty"`
	PaymentFrequency      string `bson:"paymentFrequency,omitempty"`
	PaymentReleaseTrigger string `bson:"paymentReleaseTrigger,omitempty"`
}

// derefFloat64 safely dereferences a *float64, returning 0 for nil.
func derefFloat64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// derefInt64 safely dereferences a *int64, returning 0 for nil.
func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// feePercentPtr converts a float64 to a pointer, returning nil when the value
// is 0 so the field is omitted from documents where no custom fee was configured.
func feePercentPtr(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

// feeFixedPtr converts an int64 to a pointer, returning nil when the value is 0.
func feeFixedPtr(i int64) *int64 {
	if i == 0 {
		return nil
	}
	return &i
}

func cfgDocToDomain(doc configPagamentoDocument) *domain.EventoConfigPagamento {
	methods := make([]domain.MetodoPagamento, len(doc.MetodosPagamento))
	for i, m := range doc.MetodosPagamento {
		methods[i] = domain.MetodoPagamento(m)
	}
	return &domain.EventoConfigPagamento{
		ID:                    doc.ID.Hex(),
		EventoID:              doc.EventoID,
		UsuarioID:             doc.UsuarioID,
		MetodosPagamento:      methods,
		PrazoPagamento:        doc.PrazoPagamento,
		ChavePix:              doc.ChavePix,
		InstrucoesBoleto:      doc.InstrucoesBoleto,
		CriadoEm:             doc.CriadoEm,
		UpdatedAt:             doc.UpdatedAt,
		PlatformFeePercent:    derefFloat64(doc.PlatformFeePercent),
		PspFeePercent:         derefFloat64(doc.PspFeePercent),
		PlatformFeeFixedCents: derefInt64(doc.PlatformFeeFixedCents),
		PspFeeFixedCents:      derefInt64(doc.PspFeeFixedCents),
		FeePolicyVersion:      doc.FeePolicyVersion,
		PaymentProvider:       doc.PaymentProvider,
		PaymentFrequency:      doc.PaymentFrequency,
		PaymentReleaseTrigger: doc.PaymentReleaseTrigger,
	}
}

func cfgDomainToDoc(cfg *domain.EventoConfigPagamento) configPagamentoDocument {
	methods := make([]string, len(cfg.MetodosPagamento))
	for i, m := range cfg.MetodosPagamento {
		methods[i] = string(m)
	}
	return configPagamentoDocument{
		EventoID:              cfg.EventoID,
		UsuarioID:             cfg.UsuarioID,
		MetodosPagamento:      methods,
		PrazoPagamento:        cfg.PrazoPagamento,
		ChavePix:              cfg.ChavePix,
		InstrucoesBoleto:      cfg.InstrucoesBoleto,
		CriadoEm:             cfg.CriadoEm,
		UpdatedAt:             cfg.UpdatedAt,
		PlatformFeePercent:    feePercentPtr(cfg.PlatformFeePercent),
		PspFeePercent:         feePercentPtr(cfg.PspFeePercent),
		PlatformFeeFixedCents: feeFixedPtr(cfg.PlatformFeeFixedCents),
		PspFeeFixedCents:      feeFixedPtr(cfg.PspFeeFixedCents),
		FeePolicyVersion:      cfg.FeePolicyVersion,
		PaymentProvider:       cfg.PaymentProvider,
		PaymentFrequency:      cfg.PaymentFrequency,
		PaymentReleaseTrigger: cfg.PaymentReleaseTrigger,
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

// FindByEventoID retrieves the payment config for eventID.
// Queries both snake_case (evento_id — Go-written docs) and camelCase
// (eventoId — Java-written docs) to support the shared collection.
func (r *ConfigPagamentoMongoRepository) FindByEventoID(ctx context.Context, eventoID string) (*domain.EventoConfigPagamento, error) {
	var doc configPagamentoDocument
	filter := bson.D{{Key: "$or", Value: bson.A{
		bson.D{{Key: "evento_id", Value: eventoID}},
		bson.D{{Key: "eventoId", Value: eventoID}},
	}}}
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("config_pagamento", eventoID)
		}
		return nil, apierr.Internal(err.Error())
	}
	return cfgDocToDomain(doc), nil
}

// FindAll returns all event payment configs. Used by ReaplicarFeePolicySnapshotUseCase.
func (r *ConfigPagamentoMongoRepository) FindAll(ctx context.Context) ([]*domain.EventoConfigPagamento, error) {
	cur, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var result []*domain.EventoConfigPagamento
	for cur.Next(ctx) {
		var doc configPagamentoDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		result = append(result, cfgDocToDomain(doc))
	}
	return result, nil
}

// BulkUpdateFeeFields reapplies platformFeePercent and pspFeePercent to all
// existing EventoConfigPagamento documents and sets their feePolicyVersion.
// Returns the number of documents modified.
func (r *ConfigPagamentoMongoRepository) BulkUpdateFeeFields(ctx context.Context, platformFee, pspFee float64, version string) (int64, error) {
	update := bson.M{
		"$set": bson.M{
			"platformFeePercent": platformFee,
			"pspFeePercent":      pspFee,
			"feePolicyVersion":   version,
			"updated_at":         time.Now(),
		},
	}
	res, err := r.col.UpdateMany(ctx, bson.M{}, update)
	if err != nil {
		return 0, apierr.Internal(err.Error())
	}
	return res.ModifiedCount, nil
}

func (r *ConfigPagamentoMongoRepository) Save(ctx context.Context, cfg *domain.EventoConfigPagamento) (*domain.EventoConfigPagamento, error) {
	doc := cfgDomainToDoc(cfg)
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
	doc := cfgDomainToDoc(cfg)
	doc.ID = oid
	_, err = r.col.ReplaceOne(ctx, bson.M{"_id": oid}, doc, options.Replace().SetUpsert(false))
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return cfg, nil
}

// ===================================================================
// SavedCard — collection: saved_credit_cards
// Holds a user's saved credit cards for reuse at checkout.
// user_id is stored as UUID Binary subtype 4 (Java-compatible) or ObjectID.
// ===================================================================

type savedCardDocument struct {
	ID          any        `bson:"_id,omitempty"`
	UserID      any        `bson:"user_id"`
	LastFour    string     `bson:"last_four,omitempty"`
	Brand       string     `bson:"brand,omitempty"`
	HolderName  string     `bson:"holder_name,omitempty"`
	ExpiryMonth int        `bson:"expiry_month,omitempty"`
	ExpiryYear  int        `bson:"expiry_year,omitempty"`
	IsDefault   bool       `bson:"is_default"`
	Active      bool       `bson:"active"`
	CreatedAt   time.Time  `bson:"criado_em"`
	UpdatedAt   time.Time  `bson:"atualizado_em"`
}

func savedCardDocToDomain(doc savedCardDocument) *domain.SavedCard {
	return &domain.SavedCard{
		ID:          rawIDToString(doc.ID),
		UserID:      rawIDToString(doc.UserID),
		LastFour:    doc.LastFour,
		Brand:       doc.Brand,
		HolderName:  doc.HolderName,
		ExpiryMonth: doc.ExpiryMonth,
		ExpiryYear:  doc.ExpiryYear,
		IsDefault:   doc.IsDefault,
		Active:      doc.Active,
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.UpdatedAt,
	}
}

// SavedCardMongoRepository implements portout.SavedCardRepository.
type SavedCardMongoRepository struct {
	col *mongo.Collection
}

// NewSavedCardRepository creates a SavedCardRepository backed by MongoDB.
func NewSavedCardRepository(client *Client) portout.SavedCardRepository {
	return &SavedCardMongoRepository{col: client.Collection("saved_credit_cards")}
}

// FindByUserID returns all active saved cards for a user.
func (r *SavedCardMongoRepository) FindByUserID(ctx context.Context, userID string) ([]domain.SavedCard, error) {
	filter := bson.D{
		{Key: "user_id", Value: userIDValue(userID)},
		{Key: "active", Value: bson.D{{Key: "$ne", Value: false}}},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return []domain.SavedCard{}, nil
	}
	defer cur.Close(ctx)

	result := make([]domain.SavedCard, 0)
	for cur.Next(ctx) {
		var doc savedCardDocument
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		result = append(result, *savedCardDocToDomain(doc))
	}
	return result, nil
}

// FindByID returns a single saved card by ID, verifying ownership via userID.
func (r *SavedCardMongoRepository) FindByID(ctx context.Context, id, userID string) (*domain.SavedCard, error) {
	filter := parseIDToFilter(id)
	filter = append(filter, bson.E{Key: "user_id", Value: userIDValue(userID)})

	var doc savedCardDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("saved_card", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return savedCardDocToDomain(doc), nil
}

// Save inserts a new saved card.
func (r *SavedCardMongoRepository) Save(ctx context.Context, card *domain.SavedCard) (*domain.SavedCard, error) {
	now := time.Now().UTC()
	if card.CreatedAt.IsZero() {
		card.CreatedAt = now
	}
	card.UpdatedAt = now
	card.Active = true

	doc := savedCardDocument{
		UserID:      userIDValue(card.UserID),
		LastFour:    card.LastFour,
		Brand:       card.Brand,
		HolderName:  card.HolderName,
		ExpiryMonth: card.ExpiryMonth,
		ExpiryYear:  card.ExpiryYear,
		IsDefault:   card.IsDefault,
		Active:      card.Active,
		CreatedAt:   card.CreatedAt,
		UpdatedAt:   card.UpdatedAt,
	}
	res, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, apierr.Internal("saving saved_card: " + err.Error())
	}
	card.ID = rawIDToString(res.InsertedID)
	return card, nil
}

// ClearDefault removes the is_default flag from all saved cards belonging to userID.
func (r *SavedCardMongoRepository) ClearDefault(ctx context.Context, userID string) error {
	filter := bson.D{{Key: "user_id", Value: userIDValue(userID)}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "is_default", Value: false}}}}
	_, err := r.col.UpdateMany(ctx, filter, update)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// SoftDelete marks a saved card as inactive.
func (r *SavedCardMongoRepository) SoftDelete(ctx context.Context, id, userID string) error {
	filter := parseIDToFilter(id)
	filter = append(filter, bson.E{Key: "user_id", Value: userIDValue(userID)})
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "active", Value: false},
		{Key: "atualizado_em", Value: time.Now().UTC()},
	}}}
	_, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// ===================================================================
// InstallmentQueryRepository — collection: payment_installments
// Used by the payment handler's installment query endpoints.
// ===================================================================

type financeInstallmentQueryDocument struct {
	ID            any        `bson:"_id,omitempty"`
	EventID       any        `bson:"event_id"`
	UserID        any        `bson:"user_id,omitempty"`
	ParticipantID any        `bson:"participant_id,omitempty"`
	Amount        int64      `bson:"amount"`
	Status        string     `bson:"status"`
	PaymentMethod string     `bson:"payment_method,omitempty"`
	DueDate       time.Time  `bson:"due_date,omitempty"`
	PaidAt        *time.Time `bson:"paid_at,omitempty"`
}

func financeInstallmentQueryDocToDomain(doc financeInstallmentQueryDocument) domain.Installment {
	return domain.Installment{
		ID:            rawIDToString(doc.ID),
		EventID:       rawIDToString(doc.EventID),
		UserID:        rawIDToString(doc.UserID),
		ParticipantID: rawIDToString(doc.ParticipantID),
		Amount:        doc.Amount,
		Status:        doc.Status,
		PaymentMethod: doc.PaymentMethod,
		DueDate:       doc.DueDate,
		PaidAt:        doc.PaidAt,
	}
}

// InstallmentQueryMongoRepository implements portout.InstallmentQueryRepository.
type InstallmentQueryMongoRepository struct {
	col *mongo.Collection
}

// NewInstallmentQueryRepository creates an InstallmentQueryRepository backed by MongoDB.
func NewInstallmentQueryRepository(client *Client) portout.InstallmentQueryRepository {
	return &InstallmentQueryMongoRepository{col: client.Collection("payment_installments")}
}

// FindByFilters queries installments by optional eventID, userID, and status.
func (r *InstallmentQueryMongoRepository) FindByFilters(ctx context.Context, eventID, userID, status string) ([]domain.Installment, error) {
	filter := bson.D{}
	if eventID != "" {
		filter = append(filter, bson.E{Key: "event_id", Value: UUIDStringToBinary(eventID)})
	}
	if userID != "" {
		filter = append(filter, bson.E{Key: "user_id", Value: userIDValue(userID)})
	}
	if status != "" {
		filter = append(filter, bson.E{Key: "status", Value: status})
	}

	opts := options.Find().SetSort(bson.D{{Key: "due_date", Value: 1}})
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return []domain.Installment{}, nil
	}
	defer cur.Close(ctx)

	result := make([]domain.Installment, 0)
	for cur.Next(ctx) {
		var doc financeInstallmentQueryDocument
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		result = append(result, financeInstallmentQueryDocToDomain(doc))
	}
	return result, nil
}

// FindByUserID returns all installments for a user, optionally filtered by status.
func (r *InstallmentQueryMongoRepository) FindByUserID(ctx context.Context, userID, status string) ([]domain.Installment, error) {
	filter := bson.D{{Key: "user_id", Value: userIDValue(userID)}}
	if status != "" {
		filter = append(filter, bson.E{Key: "status", Value: status})
	}

	opts := options.Find().SetSort(bson.D{{Key: "due_date", Value: 1}})
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return []domain.Installment{}, nil
	}
	defer cur.Close(ctx)

	result := make([]domain.Installment, 0)
	for cur.Next(ctx) {
		var doc financeInstallmentQueryDocument
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		result = append(result, financeInstallmentQueryDocToDomain(doc))
	}
	return result, nil
}
