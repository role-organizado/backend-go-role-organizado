package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	convitedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/convite"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ============================================================
// Convites domain — MongoDB adapter.
// Reuses existing collections shared with the Java backend:
//   - participants
//   - guests
//   - approval_items
//   - participant_credits
//   - audit_entries
//   - payment_installments (narrow port)
//   - dominios (cancellation policy projection)
// IDs follow the Java convention: UUID Binary subtype 4.
// ============================================================

// ---- Participants (convite-specific projection) -------------------------------

// ConviteParticipantMongoRepository implements port/out.ConviteParticipantRepository.
type ConviteParticipantMongoRepository struct {
	col *mongo.Collection
}

// NewConviteParticipantRepository constructs the adapter.
func NewConviteParticipantRepository(client *Client) *ConviteParticipantMongoRepository {
	return &ConviteParticipantMongoRepository{col: client.Collection("participants")}
}

type conviteParticipantDoc struct {
	ID               interface{} `bson:"_id,omitempty"`
	EventoID         bson.Binary `bson:"evento_id"`
	UsuarioID        interface{} `bson:"usuario_id,omitempty"`
	TipoParticipante string      `bson:"tipo_participante,omitempty"`
	Papel            string      `bson:"papel,omitempty"`
	Status           string      `bson:"status,omitempty"`
	Nome             string      `bson:"nome,omitempty"`
	Email            string      `bson:"email,omitempty"`
	Telefone         string      `bson:"telefone,omitempty"`
	GuestID          string      `bson:"guest_id,omitempty"`
	DataResposta     *time.Time  `bson:"data_resposta,omitempty"`
	CriadoEm         time.Time   `bson:"criado_em,omitempty"`
	AtualizadoEm     time.Time   `bson:"atualizado_em,omitempty"`
}

func conviteParticipantFromDoc(d conviteParticipantDoc) *convitedomain.Participant {
	return &convitedomain.Participant{
		ID:               rawIDToString(d.ID),
		EventoID:         uuidBinaryToString(d.EventoID),
		UsuarioID:        rawIDToString(d.UsuarioID),
		TipoParticipante: convitedomain.TipoParticipante(d.TipoParticipante),
		Papel:            convitedomain.Papel(d.Papel),
		Status:           convitedomain.ParticipantStatus(d.Status),
		Nome:             d.Nome,
		Email:            d.Email,
		Telefone:         d.Telefone,
		GuestID:          d.GuestID,
		DataResposta:     d.DataResposta,
		CriadoEm:         d.CriadoEm,
		AtualizadoEm:     d.AtualizadoEm,
	}
}

// FindByID returns the participant by its UUID id.
func (r *ConviteParticipantMongoRepository) FindByID(ctx context.Context, id string) (*convitedomain.Participant, error) {
	filter := parseIDToFilter(id)
	var doc conviteParticipantDoc
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("participant", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return conviteParticipantFromDoc(doc), nil
}

// FindByEventoIDAndUsuarioID locates a participant by composite key.
func (r *ConviteParticipantMongoRepository) FindByEventoIDAndUsuarioID(ctx context.Context, eventoID, usuarioID string) (*convitedomain.Participant, error) {
	filter := bson.D{
		{Key: "evento_id", Value: UUIDStringToBinary(eventoID)},
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
	}
	var doc conviteParticipantDoc
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, apierr.Internal(err.Error())
	}
	return conviteParticipantFromDoc(doc), nil
}

// FindByEventoIDAndPapel returns participants for an event with a specific papel.
func (r *ConviteParticipantMongoRepository) FindByEventoIDAndPapel(ctx context.Context, eventoID string, papel convitedomain.Papel) ([]convitedomain.Participant, error) {
	filter := bson.D{
		{Key: "evento_id", Value: UUIDStringToBinary(eventoID)},
		{Key: "papel", Value: string(papel)},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var out []convitedomain.Participant
	for cur.Next(ctx) {
		var doc conviteParticipantDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		out = append(out, *conviteParticipantFromDoc(doc))
	}
	return out, cur.Err()
}

// FindByTipoParticipanteAndUsuarioID returns participants matching a tipo and a
// usuario_id. Used by the guest-linking use case to find GUEST participations
// that still carry the guestId as usuario_id.
func (r *ConviteParticipantMongoRepository) FindByTipoParticipanteAndUsuarioID(ctx context.Context, tipo convitedomain.TipoParticipante, usuarioID string) ([]convitedomain.Participant, error) {
	filter := bson.D{
		{Key: "tipo_participante", Value: string(tipo)},
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var out []convitedomain.Participant
	for cur.Next(ctx) {
		var doc conviteParticipantDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		out = append(out, *conviteParticipantFromDoc(doc))
	}
	return out, cur.Err()
}

// Save upserts a participant.
func (r *ConviteParticipantMongoRepository) Save(ctx context.Context, p *convitedomain.Participant) (*convitedomain.Participant, error) {
	if p == nil {
		return nil, apierr.BadRequest("participant inválido")
	}
	now := time.Now().UTC()
	if p.CriadoEm.IsZero() {
		p.CriadoEm = now
	}
	p.AtualizadoEm = now
	id := p.ID
	if id == "" {
		id = GenerateID()
		p.ID = id
	}
	doc := bson.D{
		{Key: "_id", Value: UUIDStringToBinary(id)},
		{Key: "evento_id", Value: UUIDStringToBinary(p.EventoID)},
		{Key: "usuario_id", Value: userIDValue(p.UsuarioID)},
		{Key: "tipo_participante", Value: string(p.TipoParticipante)},
		{Key: "papel", Value: string(p.Papel)},
		{Key: "status", Value: string(p.Status)},
		{Key: "nome", Value: p.Nome},
		{Key: "email", Value: p.Email},
		{Key: "telefone", Value: p.Telefone},
		{Key: "guest_id", Value: p.GuestID},
		{Key: "criado_em", Value: p.CriadoEm},
		{Key: "atualizado_em", Value: p.AtualizadoEm},
	}
	opts := options.Replace().SetUpsert(true)
	if _, err := r.col.ReplaceOne(ctx, parseIDToFilter(id), doc, opts); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return p, nil
}

// UpdateStatus sets status (+ optional data_resposta) on a participant.
func (r *ConviteParticipantMongoRepository) UpdateStatus(ctx context.Context, participantID string, status convitedomain.ParticipantStatus, dataResposta *time.Time) error {
	now := time.Now().UTC()
	set := bson.D{
		{Key: "status", Value: string(status)},
		{Key: "atualizado_em", Value: now},
	}
	if dataResposta != nil {
		set = append(set, bson.E{Key: "data_resposta", Value: *dataResposta})
	}
	update := bson.D{{Key: "$set", Value: set}}
	if _, err := r.col.UpdateOne(ctx, parseIDToFilter(participantID), update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// BindUsuarioID sets usuario_id on a previously-unbound participant.
func (r *ConviteParticipantMongoRepository) BindUsuarioID(ctx context.Context, participantID, usuarioID string) error {
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
		{Key: "atualizado_em", Value: time.Now().UTC()},
	}}}
	if _, err := r.col.UpdateOne(ctx, parseIDToFilter(participantID), update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// ---- Guests ------------------------------------------------------------------

// ConviteGuestMongoRepository implements port/out.ConviteGuestRepository.
type ConviteGuestMongoRepository struct {
	col *mongo.Collection
}

// NewConviteGuestRepository constructs the adapter.
func NewConviteGuestRepository(client *Client) *ConviteGuestMongoRepository {
	return &ConviteGuestMongoRepository{col: client.Collection("guests")}
}

type conviteGuestDoc struct {
	ID                 interface{} `bson:"_id,omitempty"`
	Nome               string      `bson:"nome,omitempty"`
	Telefone           string      `bson:"telefone,omitempty"`
	Email              string      `bson:"email,omitempty"`
	EvoluidoParaUserID interface{} `bson:"evoluido_para_usuario_id,omitempty"`
	EvoluidoEm         *time.Time  `bson:"evoluido_em,omitempty"`
	CriadoEm           time.Time   `bson:"criado_em,omitempty"`
	AtualizadoEm       time.Time   `bson:"atualizado_em,omitempty"`
}

func conviteGuestFromDoc(d conviteGuestDoc) *convitedomain.Guest {
	return &convitedomain.Guest{
		ID:                 rawIDToString(d.ID),
		Nome:               d.Nome,
		Telefone:           d.Telefone,
		Email:              d.Email,
		EvoluidoParaUserID: rawIDToString(d.EvoluidoParaUserID),
		EvoluidoEm:         d.EvoluidoEm,
		CriadoEm:           d.CriadoEm,
		AtualizadoEm:       d.AtualizadoEm,
	}
}

// FindByID locates a guest by id.
func (r *ConviteGuestMongoRepository) FindByID(ctx context.Context, id string) (*convitedomain.Guest, error) {
	var doc conviteGuestDoc
	if err := r.col.FindOne(ctx, parseIDToFilter(id)).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("guest", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return conviteGuestFromDoc(doc), nil
}

// FindByTelefoneOrEmail returns the first guest matching either contact.
func (r *ConviteGuestMongoRepository) FindByTelefoneOrEmail(ctx context.Context, telefone, email string) (*convitedomain.Guest, error) {
	or := bson.A{}
	if telefone != "" {
		or = append(or, bson.D{{Key: "telefone", Value: telefone}})
	}
	if email != "" {
		or = append(or, bson.D{{Key: "email", Value: email}})
	}
	if len(or) == 0 {
		return nil, nil
	}
	var doc conviteGuestDoc
	err := r.col.FindOne(ctx, bson.D{{Key: "$or", Value: or}}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, apierr.Internal(err.Error())
	}
	return conviteGuestFromDoc(doc), nil
}

// Save upserts a guest.
func (r *ConviteGuestMongoRepository) Save(ctx context.Context, g *convitedomain.Guest) (*convitedomain.Guest, error) {
	if g == nil {
		return nil, apierr.BadRequest("guest inválido")
	}
	now := time.Now().UTC()
	if g.CriadoEm.IsZero() {
		g.CriadoEm = now
	}
	g.AtualizadoEm = now
	if g.ID == "" {
		g.ID = GenerateID()
	}
	doc := bson.D{
		{Key: "_id", Value: UUIDStringToBinary(g.ID)},
		{Key: "nome", Value: g.Nome},
		{Key: "telefone", Value: g.Telefone},
		{Key: "email", Value: g.Email},
		{Key: "criado_em", Value: g.CriadoEm},
		{Key: "atualizado_em", Value: g.AtualizadoEm},
	}
	opts := options.Replace().SetUpsert(true)
	if _, err := r.col.ReplaceOne(ctx, parseIDToFilter(g.ID), doc, opts); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return g, nil
}

// ---- ApprovalItems (INVITE projection) ---------------------------------------

// ConviteApprovalMongoRepository implements port/out.ConviteApprovalRepository.
type ConviteApprovalMongoRepository struct {
	col *mongo.Collection
}

// NewConviteApprovalRepository constructs the adapter.
func NewConviteApprovalRepository(client *Client) *ConviteApprovalMongoRepository {
	return &ConviteApprovalMongoRepository{col: client.Collection("approval_items")}
}

type approvalDoc struct {
	ID                      interface{}            `bson:"_id,omitempty"`
	Type                    string                 `bson:"type,omitempty"`
	Status                  string                 `bson:"status,omitempty"`
	ApproverID              interface{}            `bson:"approver_id,omitempty"`
	EventID                 bson.Binary            `bson:"event_id,omitempty"`
	TargetEntityID          interface{}            `bson:"target_entity_id,omitempty"`
	Metadata                map[string]interface{} `bson:"metadata,omitempty"`
	MaterializationStrategy string                 `bson:"materialization_strategy,omitempty"`
	ExpiresAt               *time.Time             `bson:"expires_at,omitempty"`
	ResolvedAt              *time.Time             `bson:"resolved_at,omitempty"`
	ResolvedBy              string                 `bson:"resolved_by,omitempty"`
	ResolvedNote            string                 `bson:"resolved_note,omitempty"`
	CreatedAt               time.Time              `bson:"created_at,omitempty"`
	UpdatedAt               time.Time              `bson:"updated_at,omitempty"`
}

func approvalFromDoc(d approvalDoc) *convitedomain.ApprovalItem {
	return &convitedomain.ApprovalItem{
		ID:                      rawIDToString(d.ID),
		Type:                    convitedomain.ApprovalItemType(d.Type),
		Status:                  convitedomain.ApprovalItemStatus(d.Status),
		ApproverID:              rawIDToString(d.ApproverID),
		EventID:                 uuidBinaryToString(d.EventID),
		TargetEntityID:          rawIDToString(d.TargetEntityID),
		Metadata:                d.Metadata,
		MaterializationStrategy: convitedomain.MaterializationStrategy(d.MaterializationStrategy),
		ExpiresAt:               d.ExpiresAt,
		ResolvedAt:              d.ResolvedAt,
		ResolvedBy:              d.ResolvedBy,
		ResolvedNote:            d.ResolvedNote,
		CreatedAt:               d.CreatedAt,
		UpdatedAt:               d.UpdatedAt,
	}
}

// FindByID looks up an approval item by id.
func (r *ConviteApprovalMongoRepository) FindByID(ctx context.Context, id string) (*convitedomain.ApprovalItem, error) {
	var doc approvalDoc
	if err := r.col.FindOne(ctx, parseIDToFilter(id)).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("approval", id)
		}
		return nil, apierr.Internal(err.Error())
	}
	return approvalFromDoc(doc), nil
}

// FindLatestByTargetEntityIDAndType returns the most recent approval for a target.
func (r *ConviteApprovalMongoRepository) FindLatestByTargetEntityIDAndType(ctx context.Context, targetEntityID string, t convitedomain.ApprovalItemType) (*convitedomain.ApprovalItem, error) {
	filter := bson.D{
		{Key: "target_entity_id", Value: UUIDStringToBinary(targetEntityID)},
		{Key: "type", Value: string(t)},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	var doc approvalDoc
	if err := r.col.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, apierr.Internal(err.Error())
	}
	return approvalFromDoc(doc), nil
}

// FindPendingByTargetEntityID returns a PENDING INVITE for the target (if any).
func (r *ConviteApprovalMongoRepository) FindPendingByTargetEntityID(ctx context.Context, targetEntityID string) (*convitedomain.ApprovalItem, error) {
	filter := bson.D{
		{Key: "target_entity_id", Value: UUIDStringToBinary(targetEntityID)},
		{Key: "type", Value: string(convitedomain.ApprovalTypeInvite)},
		{Key: "status", Value: string(convitedomain.ApprovalStatusPending)},
	}
	var doc approvalDoc
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, apierr.Internal(err.Error())
	}
	return approvalFromDoc(doc), nil
}

// ExistsPendingByTargetEntityID reports whether any PENDING INVITE exists.
func (r *ConviteApprovalMongoRepository) ExistsPendingByTargetEntityID(ctx context.Context, targetEntityID string) (bool, error) {
	filter := bson.D{
		{Key: "target_entity_id", Value: UUIDStringToBinary(targetEntityID)},
		{Key: "type", Value: string(convitedomain.ApprovalTypeInvite)},
		{Key: "status", Value: string(convitedomain.ApprovalStatusPending)},
	}
	count, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return false, apierr.Internal(err.Error())
	}
	return count > 0, nil
}

// Save upserts an approval item.
func (r *ConviteApprovalMongoRepository) Save(ctx context.Context, a *convitedomain.ApprovalItem) (*convitedomain.ApprovalItem, error) {
	if a == nil {
		return nil, apierr.BadRequest("approval inválido")
	}
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	if a.ID == "" {
		a.ID = GenerateID()
	}
	doc := bson.D{
		{Key: "_id", Value: UUIDStringToBinary(a.ID)},
		{Key: "type", Value: string(a.Type)},
		{Key: "status", Value: string(a.Status)},
		{Key: "approver_id", Value: userIDValue(a.ApproverID)},
		{Key: "event_id", Value: UUIDStringToBinary(a.EventID)},
		{Key: "target_entity_id", Value: UUIDStringToBinary(a.TargetEntityID)},
		{Key: "metadata", Value: a.Metadata},
		{Key: "materialization_strategy", Value: string(a.MaterializationStrategy)},
		{Key: "created_at", Value: a.CreatedAt},
		{Key: "updated_at", Value: a.UpdatedAt},
	}
	if a.ExpiresAt != nil {
		doc = append(doc, bson.E{Key: "expires_at", Value: *a.ExpiresAt})
	}
	opts := options.Replace().SetUpsert(true)
	if _, err := r.col.ReplaceOne(ctx, parseIDToFilter(a.ID), doc, opts); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	return a, nil
}

// UpdateStatus transitions an approval item to a resolved state.
func (r *ConviteApprovalMongoRepository) UpdateStatus(ctx context.Context, id string, status convitedomain.ApprovalItemStatus, resolvedBy, note string, resolvedAt time.Time) error {
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "status", Value: string(status)},
		{Key: "resolved_at", Value: resolvedAt},
		{Key: "resolved_by", Value: resolvedBy},
		{Key: "resolved_note", Value: note},
		{Key: "updated_at", Value: time.Now().UTC()},
	}}}
	if _, err := r.col.UpdateOne(ctx, parseIDToFilter(id), update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// ---- Participant Credits -----------------------------------------------------

// ConviteParticipantCreditMongoRepository implements port/out.ParticipantCreditRepository.
type ConviteParticipantCreditMongoRepository struct {
	col *mongo.Collection
}

// NewConviteParticipantCreditRepository constructs the adapter.
func NewConviteParticipantCreditRepository(client *Client) *ConviteParticipantCreditMongoRepository {
	return &ConviteParticipantCreditMongoRepository{col: client.Collection("participant_credits")}
}

// Save inserts a new credit document and returns the credit id.
func (r *ConviteParticipantCreditMongoRepository) Save(ctx context.Context, c portout.ParticipantCredit) (string, error) {
	if c.ID == "" {
		c.ID = GenerateID()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	doc := bson.D{
		{Key: "_id", Value: UUIDStringToBinary(c.ID)},
		{Key: "participant_id", Value: UUIDStringToBinary(c.ParticipantID)},
		{Key: "evento_id", Value: UUIDStringToBinary(c.EventoID)},
		{Key: "usuario_id", Value: userIDValue(c.UsuarioID)},
		{Key: "amount_cents", Value: c.AmountCents},
		{Key: "reason", Value: c.Reason},
		{Key: "status", Value: c.Status},
		{Key: "created_at", Value: c.CreatedAt},
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return "", apierr.Internal(err.Error())
	}
	return c.ID, nil
}

// ---- Audit Entries -----------------------------------------------------------

// ConviteAuditMongoRepository implements port/out.ConviteAuditRepository.
type ConviteAuditMongoRepository struct {
	col *mongo.Collection
}

// NewConviteAuditRepository constructs the adapter.
func NewConviteAuditRepository(client *Client) *ConviteAuditMongoRepository {
	return &ConviteAuditMongoRepository{col: client.Collection("audit_entries")}
}

// Save inserts an audit entry.
func (r *ConviteAuditMongoRepository) Save(ctx context.Context, e portout.ConviteAuditEntry) error {
	doc := bson.D{
		{Key: "_id", Value: UUIDStringToBinary(GenerateID())},
		{Key: "acao", Value: e.Acao},
		{Key: "evento_id", Value: UUIDStringToBinary(e.EventoID)},
		{Key: "admin_id", Value: userIDValue(e.AdminID)},
		{Key: "detalhes", Value: e.Detalhes},
		{Key: "ocorrido_em", Value: e.OcorridoEm},
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// ---- Cancellation policy via dominios ----------------------------------------

// ConvitePoliticaMongoRepository implements port/out.ConvitePoliticaCancelamentoRepository.
type ConvitePoliticaMongoRepository struct {
	col *mongo.Collection
}

// NewConvitePoliticaRepository constructs the adapter.
func NewConvitePoliticaRepository(client *Client) *ConvitePoliticaMongoRepository {
	return &ConvitePoliticaMongoRepository{col: client.Collection("dominios")}
}

// FindByChave loads a politica_cancelamento dominio by chave; returns nil
// when the dominio isn't registered (caller falls back to defaults).
func (r *ConvitePoliticaMongoRepository) FindByChave(ctx context.Context, chave string) (*convitedomain.CancellationPolicy, error) {
	filter := bson.D{
		{Key: "categoria", Value: "politica_cancelamento"},
		{Key: "chave", Value: chave},
	}
	var doc struct {
		Chave string `bson:"chave"`
		Valor any    `bson:"valor"`
	}
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, apierr.Internal(err.Error())
	}
	policy := &convitedomain.CancellationPolicy{Chave: doc.Chave}
	// Valor is expected to be an array of {dias_minimos:int, refund_percent:float, label:string}
	if list, ok := doc.Valor.(bson.A); ok {
		for _, item := range list {
			m, ok := item.(bson.M)
			if !ok {
				continue
			}
			tier := convitedomain.CancellationTier{}
			if v, ok := m["dias_minimos"]; ok {
				tier.DiasMinimos = int(ToInt64(v))
			}
			if v, ok := m["refund_percent"]; ok {
				switch x := v.(type) {
				case float64:
					tier.RefundPercent = x
				case float32:
					tier.RefundPercent = float64(x)
				}
			}
			if v, ok := m["label"]; ok {
				if s, ok := v.(string); ok {
					tier.Label = s
				}
			}
			policy.Tiers = append(policy.Tiers, tier)
		}
	}
	return policy, nil
}

// ---- Installments (narrow port projection) -----------------------------------

// ConviteInstallmentMongoRepository implements port/out.ConviteInstallmentRepository.
type ConviteInstallmentMongoRepository struct {
	col *mongo.Collection
}

// NewConviteInstallmentRepository constructs the adapter.
func NewConviteInstallmentRepository(client *Client) *ConviteInstallmentMongoRepository {
	return &ConviteInstallmentMongoRepository{col: client.Collection("payment_installments")}
}

// FindByParticipantID lists all installments for a participant in an event.
func (r *ConviteInstallmentMongoRepository) FindByParticipantID(ctx context.Context, eventoID, participantID string) ([]portout.ConviteInstallment, error) {
	filter := bson.D{
		{Key: "event_id", Value: UUIDStringToBinary(eventoID)},
		{Key: "participant_id", Value: UUIDStringToBinary(participantID)},
	}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, apierr.Internal(err.Error())
	}
	defer cur.Close(ctx)
	var out []portout.ConviteInstallment
	for cur.Next(ctx) {
		var d struct {
			ID     interface{} `bson:"_id"`
			Status string      `bson:"status"`
			Amount int64       `bson:"amount"`
			PaidAt *time.Time  `bson:"paid_at,omitempty"`
		}
		if err := cur.Decode(&d); err != nil {
			return nil, apierr.Internal(err.Error())
		}
		out = append(out, portout.ConviteInstallment{
			ID:          rawIDToString(d.ID),
			Status:      d.Status,
			AmountCents: d.Amount,
			PaidAt:      d.PaidAt,
		})
	}
	return out, cur.Err()
}

// CancelPendingByParticipantID transitions PENDING/OVERDUE installments to CANCELLED.
func (r *ConviteInstallmentMongoRepository) CancelPendingByParticipantID(ctx context.Context, eventoID, participantID string) (int64, error) {
	filter := bson.D{
		{Key: "event_id", Value: UUIDStringToBinary(eventoID)},
		{Key: "participant_id", Value: UUIDStringToBinary(participantID)},
		{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"PENDING", "OVERDUE"}}}},
	}
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "status", Value: "CANCELLED"},
		{Key: "updated_at", Value: time.Now().UTC()},
	}}}
	res, err := r.col.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, apierr.Internal(err.Error())
	}
	return res.ModifiedCount, nil
}

// ---- No-op notification port -------------------------------------------------

// NoopConviteNotificationAdapter is a default ConviteNotificationPort that does
// not enqueue anything. It is used when SQS is disabled — the use case still
// succeeds but returns an empty messageId.
type NoopConviteNotificationAdapter struct{}

// NewNoopConviteNotificationAdapter constructs the adapter.
func NewNoopConviteNotificationAdapter() *NoopConviteNotificationAdapter {
	return &NoopConviteNotificationAdapter{}
}

// PublicarConvite implements port/out.ConviteNotificationPort.
func (a *NoopConviteNotificationAdapter) PublicarConvite(_ context.Context, _ portout.ConvitePublishInput) (string, error) {
	return "", nil
}
