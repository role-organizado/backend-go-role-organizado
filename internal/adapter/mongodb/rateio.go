package mongodb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- rateioDocument — matches Java's MongoDB schema ----
// Required: evento_id (binData), criado_em
// Enum: tipo_cobranca (PERCENTUAL/DIVISAO/ITENS/FIXO), status (ATIVO/CANCELADO)

type rateioDocument struct {
	ID                   interface{} `bson:"_id,omitempty"`
	EventoID             bson.Binary `bson:"evento_id"`                        // MUST be binData per schema
	UsuarioIDResponsavel interface{} `bson:"usuario_id_responsavel,omitempty"` // ObjectID or binData depending on user origin
	Nome                 string      `bson:"nome,omitempty"`
	TipoCobranca         string      `bson:"tipo_cobranca"`                   // enum: PERCENTUAL/DIVISAO/ITENS/FIXO
	ValorRateado         int64       `bson:"valor_rateado"`                   // long in Java
	Status               string      `bson:"status"`                          // enum: ATIVO/CANCELADO
	PendenteRecalculo    bool        `bson:"pendente_recalculo"`
	CriadoEm            time.Time   `bson:"criado_em"`
	AtualizadoEm        time.Time   `bson:"atualizado_em,omitempty"`          // Java: atualizado_em
}

// statusToMongo maps Go domain status to MongoDB enum.
func statusToMongo(s domain.StatusRateio) string {
	switch s {
	case domain.StatusRateioFechado:
		return "CANCELADO"
	default:
		return "ATIVO"
	}
}

// statusFromMongo maps MongoDB enum to Go domain status.
func statusFromMongo(s string) domain.StatusRateio {
	switch s {
	case "CANCELADO":
		return domain.StatusRateioFechado
	default:
		return domain.StatusRateioAberto
	}
}

func rateioDocFromDomain(r *domain.Rateio) rateioDocument {
	eventoIDHex := r.EventoID
	eventoIDBin := UUIDStringToBinary(eventoIDHex)

	doc := rateioDocument{
		EventoID:          eventoIDBin,
		Nome:              r.Descricao,
		TipoCobranca:      string(r.Tipo),
		ValorRateado:      int64(r.ValorTotal),
		Status:            statusToMongo(r.Status),
		PendenteRecalculo: false,
		CriadoEm:         r.CriadoEm,
		AtualizadoEm:     r.UpdatedAt,
	}
	if r.UsuarioID != "" {
		doc.UsuarioIDResponsavel = userIDValue(r.UsuarioID)
	}
	return doc
}

func ratiodocToDomain(doc rateioDocument) *domain.Rateio {
	id := rawIDToString(doc.ID)
	var usuarioID string
	if doc.UsuarioIDResponsavel != nil {
		usuarioID = rawIDToString(doc.UsuarioIDResponsavel)
	}
	return &domain.Rateio{
		ID:        id,
		EventoID:  uuidBinaryToString(doc.EventoID),
		UsuarioID: usuarioID,
		Tipo:      domain.TipoRateio(doc.TipoCobranca),
		Status:    statusFromMongo(doc.Status),
		Descricao: doc.Nome,
		ValorTotal: float64(doc.ValorRateado),
		CriadoEm:  doc.CriadoEm,
		UpdatedAt: doc.AtualizadoEm,
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
	filter := parseIDToFilter(id)
	var doc rateioDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("rateio", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return ratiodocToDomain(doc), nil
}

func (r *RateioMongoRepository) FindByEventoID(ctx context.Context, eventoID string) ([]domain.Rateio, error) {
	// eventoID must be a valid UUID string — it is stored as bson.Binary subtype 4
	// in the Java-compatible schema. UUIDStringToBinary produces the same binary
	// on every call for a given valid UUID, ensuring the filter matches the stored value.
	filter := bson.D{{Key: "evento_id", Value: UUIDStringToBinary(eventoID)}}
	cur, err := r.col.Find(ctx, filter)
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
	if err := cur.Err(); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return result, nil
}

func (r *RateioMongoRepository) FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Rateio, int64, error) {
	filter := bson.D{{Key: "usuario_id_responsavel", Value: userIDValue(usuarioID)}}
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
	// Use UUID binary for _id to match Java schema
	newUUID := uuid.New()
	b := [16]byte(newUUID)
	newID := bson.Binary{Subtype: 0x04, Data: b[:]}

	now := time.Now().UTC()
	if rat.CriadoEm.IsZero() {
		rat.CriadoEm = now
	}
	rat.UpdatedAt = now

	doc := rateioDocFromDomain(rat)
	doc.ID = newID
	doc.AtualizadoEm = now

	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, apierr.Internal("saving rateio: " + err.Error())
	}
	rat.ID = rawIDToString(newID)
	return rat, nil
}

func (r *RateioMongoRepository) Update(ctx context.Context, rat *domain.Rateio) (*domain.Rateio, error) {
	filter := parseIDToFilter(rat.ID)
	now := time.Now().UTC()
	rat.UpdatedAt = now

	setDoc := bson.D{
		{Key: "tipo_cobranca", Value: string(rat.Tipo)},
		{Key: "valor_rateado", Value: int64(rat.ValorTotal)},
		{Key: "status", Value: statusToMongo(rat.Status)},
		{Key: "nome", Value: rat.Descricao},
		{Key: "atualizado_em", Value: now},
	}
	update := bson.D{{Key: "$set", Value: setDoc}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return nil, apierr.NotFound("rateio", rat.ID)
	}
	return rat, nil
}

func (r *RateioMongoRepository) DeleteByID(ctx context.Context, id string) error {
	filter := parseIDToFilter(id)
	res, err := r.col.DeleteOne(ctx, filter)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("rateio", id)
	}
	return nil
}

// ---- rateioFechamentoDocument ----
// rateio_fechamentos has no strict schema validator — uses ObjectID for _id.

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
