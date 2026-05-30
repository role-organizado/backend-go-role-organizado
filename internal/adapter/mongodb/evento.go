package mongodb

import (
	"context"
	"encoding/base64"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

const colEventos = "eventos"

// eventoDocument matches Java's MongoDB schema for the 'eventos' collection.
// _id can be ObjectID (Go-created) or UUID binary (Java-created) — use interface{}.
type eventoDocument struct {
	ID                   interface{} `bson:"_id,omitempty"`
	Nome                 string      `bson:"nome"`
	Tipo                 string      `bson:"tipo"`
	DataInicio           time.Time   `bson:"data_inicio"`            // Java: data_inicio (not "data")
	DataFim              *time.Time  `bson:"data_fim,omitempty"`
	UsuarioIDResponsavel interface{} `bson:"usuario_id_responsavel"` // UUID binary in Java schema
	Descricao            string      `bson:"descricao,omitempty"`
	Local                string      `bson:"local,omitempty"`
	Status               string      `bson:"status"`
	LimiteConvidados     *int32      `bson:"limite_convidados,omitempty"`
	PoliticaConvidados   string      `bson:"politica_convidados,omitempty"`
	PoliticaCancelamento string      `bson:"politica_cancelamento,omitempty"`
	RateiosHabilitado    bool        `bson:"rateios_habilitado,omitempty"`
	PagamentosHabilitado bool        `bson:"pagamentos_habilitado,omitempty"`
	ImageURL             string      `bson:"image_url,omitempty"`
	CriadoEm            time.Time   `bson:"criado_em,omitempty"`
	AtualizadoEm        time.Time   `bson:"atualizado_em,omitempty"` // Java: atualizado_em (not "updated_at")
}

// EventoRepository implements portout.EventoRepository using MongoDB.
type EventoRepository struct {
	col *mongo.Collection
}

// NewEventoRepository creates a new EventoRepository.
func NewEventoRepository(client *Client) *EventoRepository {
	return &EventoRepository{col: client.Collection(colEventos)}
}

// FindByID retrieves an event by its ID (handles ObjectID hex and UUID string).
func (r *EventoRepository) FindByID(ctx context.Context, id string) (*domain.Evento, error) {
	filter := parseIDToFilter(id)
	var doc eventoDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
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
	filter := bson.D{{Key: "usuario_id_responsavel", Value: uuidStringToBinary(usuarioID)}}
	return r.findPaginated(ctx, filter, page, pageSize)
}

// FindByUsuarioIDCursor returns a cursor-paginated list of events for the given user.
// The cursor is a base64-encoded skip offset, allowing stable forward-only pagination.
func (r *EventoRepository) FindByUsuarioIDCursor(ctx context.Context, usuarioID string, filtros portout.EventoQueryFiltros) (portout.EventosCursorPage, error) {
	limit := filtros.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Decode cursor → skip offset
	skip := int64(0)
	if filtros.Cursor != nil && *filtros.Cursor != "" {
		if decoded, err := base64.StdEncoding.DecodeString(*filtros.Cursor); err == nil {
			if off, err := strconv.ParseInt(string(decoded), 10, 64); err == nil && off > 0 {
				skip = off
			}
		}
	}

	// Build filter
	filter := bson.D{{Key: "usuario_id_responsavel", Value: uuidStringToBinary(usuarioID)}}
	if filtros.Status != nil {
		filter = append(filter, bson.E{Key: "status", Value: *filtros.Status})
	}
	if filtros.Tipo != nil {
		filter = append(filter, bson.E{Key: "tipo", Value: *filtros.Tipo})
	}
	if filtros.DataInicioGte != nil || filtros.DataInicioLte != nil {
		rangeDoc := bson.D{}
		if filtros.DataInicioGte != nil {
			rangeDoc = append(rangeDoc, bson.E{Key: "$gte", Value: *filtros.DataInicioGte})
		}
		if filtros.DataInicioLte != nil {
			rangeDoc = append(rangeDoc, bson.E{Key: "$lte", Value: *filtros.DataInicioLte})
		}
		filter = append(filter, bson.E{Key: "data_inicio", Value: rangeDoc})
	}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return portout.EventosCursorPage{}, apierr.Internal(err.Error())
	}

	// Fetch limit+1 to detect hasNextPage
	fetchLimit := int64(limit + 1)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(fetchLimit).
		SetSort(bson.D{{Key: "data_inicio", Value: -1}, {Key: "_id", Value: -1}})

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return portout.EventosCursorPage{}, apierr.Internal(err.Error())
	}
	defer cursor.Close(ctx)

	var docs []eventoDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return portout.EventosCursorPage{}, apierr.Internal(err.Error())
	}

	hasNextPage := len(docs) > limit
	if hasNextPage {
		docs = docs[:limit]
	}

	eventos := make([]domain.Evento, len(docs))
	for i, d := range docs {
		eventos[i] = eventoFromDoc(d)
	}

	var nextCursor *string
	if hasNextPage {
		nextOffset := skip + int64(limit)
		encoded := base64.StdEncoding.EncodeToString([]byte(strconv.FormatInt(nextOffset, 10)))
		nextCursor = &encoded
	}

	return portout.EventosCursorPage{
		Eventos:     eventos,
		Total:       total,
		NextCursor:  nextCursor,
		HasNextPage: hasNextPage,
		Limit:       limit,
	}, nil
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
	opts := options.Find().SetSkip(skip).SetLimit(int64(pageSize)).SetSort(bson.D{{Key: "data_inicio", Value: -1}})
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

// Save inserts a new event. Uses UUID binary for _id (matches Java schema); stores usuario_id_responsavel as UUID binary.
func (r *EventoRepository) Save(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	// Use UUID binary for _id so that the ID can be used as binData in rateio.evento_id
	newID := uuidStringToBinary(uuid.New().String())
	now := time.Now().UTC()
	status := string(e.Status)
	if status == "" {
		status = "CONFIRMADO"
	}
	doc := eventoDocument{
		ID:                   newID,
		Nome:                 e.Nome,
		Tipo:                 e.Tipo,
		DataInicio:           e.Data,
		UsuarioIDResponsavel: uuidStringToBinary(e.UsuarioID),
		Descricao:            e.Descricao,
		Local:                e.Local,
		Status:               status,
		PoliticaConvidados:   e.PoliticaConvidados,
		PoliticaCancelamento: e.PoliticaCancelamento,
		RateiosHabilitado:    e.RateiosHabilitado,
		PagamentosHabilitado: e.PagamentosHabilitado,
		CriadoEm:            now,
		AtualizadoEm:        now,
	}
	if e.LimiteConvidados != nil {
		v := int32(*e.LimiteConvidados)
		doc.LimiteConvidados = &v
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	saved := eventoFromDoc(doc)
	return &saved, nil
}

// Update updates an existing event using $set to preserve unknown fields.
func (r *EventoRepository) Update(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	filter := parseIDToFilter(e.ID)
	now := time.Now().UTC()
	setDoc := bson.D{
		{Key: "nome", Value: e.Nome},
		{Key: "tipo", Value: e.Tipo},
		{Key: "data_inicio", Value: e.Data},
		{Key: "descricao", Value: e.Descricao},
		{Key: "local", Value: e.Local},
		{Key: "status", Value: string(e.Status)},
		{Key: "politica_convidados", Value: e.PoliticaConvidados},
		{Key: "politica_cancelamento", Value: e.PoliticaCancelamento},
		{Key: "rateios_habilitado", Value: e.RateiosHabilitado},
		{Key: "pagamentos_habilitado", Value: e.PagamentosHabilitado},
		{Key: "atualizado_em", Value: now},
	}
	update := bson.D{{Key: "$set", Value: setDoc}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return nil, apierr.NotFound("evento", e.ID)
	}
	e.UpdatedAt = now
	return e, nil
}

// DeleteByID removes an event by its ID.
func (r *EventoRepository) DeleteByID(ctx context.Context, id string) error {
	filter := parseIDToFilter(id)
	res, err := r.col.DeleteOne(ctx, filter)
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
	var limite *int
	if doc.LimiteConvidados != nil {
		v := int(*doc.LimiteConvidados)
		limite = &v
	}
	return domain.Evento{
		ID:                   rawIDToString(doc.ID),
		UsuarioID:            rawIDToString(doc.UsuarioIDResponsavel),
		Nome:                 doc.Nome,
		Tipo:                 doc.Tipo,
		Data:                 doc.DataInicio,
		Descricao:            doc.Descricao,
		Local:                doc.Local,
		Status:               domain.EventoStatus(doc.Status),
		PoliticaConvidados:   doc.PoliticaConvidados,
		LimiteConvidados:     limite,
		RateiosHabilitado:    doc.RateiosHabilitado,
		PagamentosHabilitado: doc.PagamentosHabilitado,
		PoliticaCancelamento: doc.PoliticaCancelamento,
		CriadoEm:             doc.CriadoEm,
		UpdatedAt:            doc.AtualizadoEm,
	}
}
