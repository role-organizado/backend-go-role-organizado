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

// convidadoDoc is the MongoDB BSON representation of a Convidado (guest).
type convidadoDoc struct {
	Telefone string `bson:"telefone"`
	Nome     string `bson:"nome,omitempty"`
}

// eventoEnderecoDoc is the BSON sub-doc for the structured event address.
type eventoEnderecoDoc struct {
	Rua         string   `bson:"rua,omitempty"`
	Numero      string   `bson:"numero,omitempty"`
	Complemento string   `bson:"complemento,omitempty"`
	Bairro      string   `bson:"bairro,omitempty"`
	Cidade      string   `bson:"cidade,omitempty"`
	Estado      string   `bson:"estado,omitempty"`
	Cep         string   `bson:"cep,omitempty"`
	PlaceID     string   `bson:"place_id,omitempty"`
	Latitude    *float64 `bson:"latitude,omitempty"`
	Longitude   *float64 `bson:"longitude,omitempty"`
}

// eventoImagemDoc is the BSON sub-doc for an event image.
type eventoImagemDoc struct {
	URL          string    `bson:"url"`
	Ordem        int       `bson:"ordem"`
	Tipo         string    `bson:"tipo,omitempty"`
	AdicionadaEm time.Time `bson:"adicionada_em,omitempty"`
}

// eventoDocument matches Java's MongoDB schema for the 'eventos' collection.
// _id can be ObjectID (Go-created) or UUID binary (Java-created) — use interface{}.
type eventoDocument struct {
	ID                    interface{}       `bson:"_id,omitempty"`
	Nome                  string            `bson:"nome"`
	Tipo                  string            `bson:"tipo"`
	DataInicio            time.Time         `bson:"data_inicio"` // Java: data_inicio (not "data")
	DataFim               *time.Time        `bson:"data_fim,omitempty"`
	UsuarioIDResponsavel  interface{}       `bson:"usuario_id_responsavel"` // UUID binary in Java schema
	Descricao             string            `bson:"descricao,omitempty"`
	Local                 string            `bson:"local,omitempty"`
	Status                string            `bson:"status"`
	LimiteConvidados      *int32            `bson:"limite_convidados,omitempty"`
	PoliticaConvidados    string            `bson:"politica_convidados,omitempty"`
	PoliticaCancelamento  string            `bson:"politica_cancelamento,omitempty"`
	RateiosHabilitado     bool              `bson:"rateios_habilitado,omitempty"`
	PagamentosHabilitado  bool              `bson:"pagamentos_habilitado,omitempty"`
	ImageURL              string            `bson:"image_url,omitempty"`
	Imagens               []eventoImagemDoc `bson:"imagens,omitempty"`
	Endereco              *eventoEnderecoDoc      `bson:"endereco,omitempty"`
	Fase                  string            `bson:"fase,omitempty"`
	PaymentReleaseTrigger string            `bson:"payment_release_trigger,omitempty"`
	Convidados            []convidadoDoc    `bson:"convidados,omitempty"`
	ModulosAtivos         []string          `bson:"modulos_ativos,omitempty"`
	ConfiguracaoNicho     map[string]any    `bson:"configuracao_nicho,omitempty"`
	CriadoEm              time.Time         `bson:"criado_em,omitempty"`
	AtualizadoEm          time.Time         `bson:"atualizado_em,omitempty"` // Java: atualizado_em (not "updated_at")
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
	filter := bson.D{{Key: "usuario_id_responsavel", Value: userIDValue(usuarioID)}}
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
	filter := bson.D{{Key: "usuario_id_responsavel", Value: userIDValue(usuarioID)}}
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
	newID := UUIDStringToBinary(uuid.New().String())
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
		UsuarioIDResponsavel: userIDValue(e.UsuarioID),
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

// AddConvidados appends the given convidados to an event's guest list using $push/$each.
func (r *EventoRepository) AddConvidados(ctx context.Context, eventoID string, convidados []domain.Convidado) error {
	filter := parseIDToFilter(eventoID)
	docs := make([]convidadoDoc, len(convidados))
	for i, c := range convidados {
		docs[i] = convidadoDoc{Telefone: c.Telefone, Nome: c.Nome}
	}
	update := bson.D{{Key: "$push", Value: bson.D{
		{Key: "convidados", Value: bson.D{
			{Key: "$each", Value: docs},
		}},
	}}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("evento", eventoID)
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
	imgs := make([]domain.EventoImagem, len(doc.Imagens))
	for i, d := range doc.Imagens {
		imgs[i] = domain.EventoImagem{
			URL:          d.URL,
			Ordem:        d.Ordem,
			Tipo:         d.Tipo,
			AdicionadaEm: d.AdicionadaEm,
		}
	}
	var end *domain.EventoEndereco
	if doc.Endereco != nil {
		end = &domain.EventoEndereco{
			Rua:         doc.Endereco.Rua,
			Numero:      doc.Endereco.Numero,
			Complemento: doc.Endereco.Complemento,
			Bairro:      doc.Endereco.Bairro,
			Cidade:      doc.Endereco.Cidade,
			Estado:      doc.Endereco.Estado,
			Cep:         doc.Endereco.Cep,
			PlaceID:     doc.Endereco.PlaceID,
			Latitude:    doc.Endereco.Latitude,
			Longitude:   doc.Endereco.Longitude,
		}
	}
	return domain.Evento{
		ID:                    rawIDToString(doc.ID),
		UsuarioID:             rawIDToString(doc.UsuarioIDResponsavel),
		Nome:                  doc.Nome,
		Tipo:                  doc.Tipo,
		Data:                  doc.DataInicio,
		DataFim:               doc.DataFim,
		Descricao:             doc.Descricao,
		Local:                 doc.Local,
		Status:                domain.EventoStatus(doc.Status),
		PoliticaConvidados:    doc.PoliticaConvidados,
		LimiteConvidados:      limite,
		RateiosHabilitado:     doc.RateiosHabilitado,
		PagamentosHabilitado:  doc.PagamentosHabilitado,
		PoliticaCancelamento:  doc.PoliticaCancelamento,
		ModulosAtivos:         doc.ModulosAtivos,
		ConfiguracaoNicho:     doc.ConfiguracaoNicho,
		Fase:                  domain.EventoFase(doc.Fase),
		PaymentReleaseTrigger: doc.PaymentReleaseTrigger,
		Endereco:              end,
		ImageURL:              doc.ImageURL,
		Imagens:               imgs,
		CriadoEm:              doc.CriadoEm,
		UpdatedAt:             doc.AtualizadoEm,
	}
}

// FindAllByIDs returns the events whose IDs are in the given list. Java-created
// IDs are stored as UUID Binary subtype 4; Go-created may be ObjectIDs. We
// query with an $in containing every viable representation.
func (r *EventoRepository) FindAllByIDs(ctx context.Context, ids []string) ([]domain.Evento, error) {
	if len(ids) == 0 {
		return []domain.Evento{}, nil
	}
	values := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		if oid, err := bson.ObjectIDFromHex(id); err == nil {
			values = append(values, oid)
			continue
		}
		parts := id
		_ = parts
		if u, err := uuid.Parse(id); err == nil {
			b := [16]byte(u)
			values = append(values, bson.Binary{Subtype: 0x04, Data: b[:]})
			continue
		}
		values = append(values, id)
	}
	filter := bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: values}}}}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var docs []eventoDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	result := make([]domain.Evento, len(docs))
	for i, d := range docs {
		result[i] = eventoFromDoc(d)
	}
	return result, nil
}

// UpdateFase atomically updates only the fase field and atualizado_em.
func (r *EventoRepository) UpdateFase(ctx context.Context, id string, fase domain.EventoFase) error {
	filter := parseIDToFilter(id)
	now := time.Now().UTC()
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "fase", Value: string(fase)},
		{Key: "atualizado_em", Value: now},
	}}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("evento", id)
	}
	return nil
}

// UpdatePoliticaConvidados atomically updates politica_convidados field.
func (r *EventoRepository) UpdatePoliticaConvidados(ctx context.Context, id, politica string) error {
	filter := parseIDToFilter(id)
	now := time.Now().UTC()
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "politica_convidados", Value: politica},
		{Key: "atualizado_em", Value: now},
	}}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("evento", id)
	}
	return nil
}

// AddImagens appends a list of images to the event's imagens array using $push/$each.
func (r *EventoRepository) AddImagens(ctx context.Context, id string, imagens []domain.EventoImagem) error {
	if len(imagens) == 0 {
		return nil
	}
	filter := parseIDToFilter(id)
	docs := make([]eventoImagemDoc, len(imagens))
	for i, im := range imagens {
		docs[i] = eventoImagemDoc{
			URL:          im.URL,
			Ordem:        im.Ordem,
			Tipo:         im.Tipo,
			AdicionadaEm: im.AdicionadaEm,
		}
	}
	now := time.Now().UTC()
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "imagens", Value: bson.D{{Key: "$each", Value: docs}}}}},
		{Key: "$set", Value: bson.D{{Key: "atualizado_em", Value: now}}},
	}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return apierr.Internal(err.Error())
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("evento", id)
	}
	return nil
}

// UpdateDetalhes patches editable detail fields and returns the updated event.
func (r *EventoRepository) UpdateDetalhes(ctx context.Context, e *domain.Evento) (*domain.Evento, error) {
	filter := parseIDToFilter(e.ID)
	now := time.Now().UTC()
	setDoc := bson.D{
		{Key: "nome", Value: e.Nome},
		{Key: "tipo", Value: e.Tipo},
		{Key: "descricao", Value: e.Descricao},
		{Key: "local", Value: e.Local},
		{Key: "data_inicio", Value: e.Data},
		{Key: "atualizado_em", Value: now},
	}
	if e.DataFim != nil {
		setDoc = append(setDoc, bson.E{Key: "data_fim", Value: *e.DataFim})
	}
	if e.Endereco != nil {
		setDoc = append(setDoc, bson.E{Key: "endereco", Value: eventoEnderecoDoc{
			Rua:         e.Endereco.Rua,
			Numero:      e.Endereco.Numero,
			Complemento: e.Endereco.Complemento,
			Bairro:      e.Endereco.Bairro,
			Cidade:      e.Endereco.Cidade,
			Estado:      e.Endereco.Estado,
			Cep:         e.Endereco.Cep,
			PlaceID:     e.Endereco.PlaceID,
			Latitude:    e.Endereco.Latitude,
			Longitude:   e.Endereco.Longitude,
		}})
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
