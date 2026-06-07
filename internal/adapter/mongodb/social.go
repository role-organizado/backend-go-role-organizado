package mongodb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

const colSocialFeatures = "evento_social_features"

// socialFeaturesDocument matches Java's MongoDB schema for evento_social_features.
// Field names use camelCase to preserve BSON compatibility with the Java backend.
// _id uses any to support ObjectID/UUID-binary/string created by Java or Go.
type socialFeaturesDocument struct {
	ID                any                    `bson:"_id,omitempty"`
	EventoID          string                 `bson:"eventoId"`
	DressCode         *social.DressCode      `bson:"dressCode,omitempty"`
	Playlists         []social.PlaylistLink  `bson:"playlists,omitempty"`
	BringList         []social.BringListItem `bson:"bringList,omitempty"`
	CheckinHabilitado bool                   `bson:"checkinHabilitado"`
	Checkins          []social.Checkin       `bson:"checkins,omitempty"`
	AlbumLinks        []social.AlbumLink     `bson:"albumLinks,omitempty"`
	CriadoEm          time.Time              `bson:"criadoEm"`
	AtualizadoEm      time.Time              `bson:"atualizadoEm"`
}

// socialFromDoc converts a MongoDB document to the domain entity.
// Nil slices are normalised to empty slices so callers never deal with nil arrays.
func socialFromDoc(doc socialFeaturesDocument) social.EventoSocialFeatures {
	playlists := doc.Playlists
	if playlists == nil {
		playlists = []social.PlaylistLink{}
	}
	bringList := doc.BringList
	if bringList == nil {
		bringList = []social.BringListItem{}
	}
	checkins := doc.Checkins
	if checkins == nil {
		checkins = []social.Checkin{}
	}
	albumLinks := doc.AlbumLinks
	if albumLinks == nil {
		albumLinks = []social.AlbumLink{}
	}
	return social.EventoSocialFeatures{
		ID:                rawIDToString(doc.ID),
		EventoID:          doc.EventoID,
		DressCode:         doc.DressCode,
		Playlists:         playlists,
		BringList:         bringList,
		CheckinHabilitado: doc.CheckinHabilitado,
		Checkins:          checkins,
		AlbumLinks:        albumLinks,
		CriadoEm:          doc.CriadoEm,
		AtualizadoEm:      doc.AtualizadoEm,
	}
}

// newSocialDefaults returns the $setOnInsert fields used when lazily creating a document.
func newSocialDefaults(eventoID string, now time.Time) bson.D {
	return bson.D{
		{Key: "_id", Value: uuid.New().String()},
		{Key: "eventoId", Value: eventoID},
		{Key: "checkinHabilitado", Value: false},
		{Key: "criadoEm", Value: now},
	}
}

// ---- SocialFeaturesRepository ----

// SocialFeaturesRepository implements out.SocialFeaturesRepository using MongoDB.
// The collection 'evento_social_features' is shared with the Java backend.
type SocialFeaturesRepository struct {
	col *mongo.Collection
}

// NewSocialFeaturesRepository creates a new SocialFeaturesRepository.
func NewSocialFeaturesRepository(client *Client) *SocialFeaturesRepository {
	return &SocialFeaturesRepository{col: client.Collection(colSocialFeatures)}
}

// FindByEventoID returns the social features document or nil (no error) when not found.
func (r *SocialFeaturesRepository) FindByEventoID(ctx context.Context, eventoID string) (*social.EventoSocialFeatures, error) {
	var doc socialFeaturesDocument
	err := r.col.FindOne(ctx, bson.D{{Key: "eventoId", Value: eventoID}}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, apierr.Internal(err.Error())
	}
	sf := socialFromDoc(doc)
	return &sf, nil
}

// FindOrCreate returns the existing document or atomically creates an empty one via upsert.
func (r *SocialFeaturesRepository) FindOrCreate(ctx context.Context, eventoID string) (*social.EventoSocialFeatures, error) {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$setOnInsert", Value: newSocialDefaults(eventoID, now)},
	}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var doc socialFeaturesDocument
	if err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc); err != nil {
		return nil, apierr.Internal(err.Error())
	}
	sf := socialFromDoc(doc)
	return &sf, nil
}

// SetDressCode sets or replaces the dress code (upsert — lazy creates the document).
func (r *SocialFeaturesRepository) SetDressCode(ctx context.Context, eventoID string, dc *social.DressCode) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "dressCode", Value: dc},
			{Key: "atualizadoEm", Value: now},
		}},
		{Key: "$setOnInsert", Value: newSocialDefaults(eventoID, now)},
	}
	opts := options.UpdateOne().SetUpsert(true)
	if _, err := r.col.UpdateOne(ctx, filter, update, opts); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// AddPlaylist appends a playlist link to the event's playlist list (upsert — lazy creates).
func (r *SocialFeaturesRepository) AddPlaylist(ctx context.Context, eventoID string, p social.PlaylistLink) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "playlists", Value: p}}},
		{Key: "$set", Value: bson.D{{Key: "atualizadoEm", Value: now}}},
		{Key: "$setOnInsert", Value: newSocialDefaults(eventoID, now)},
	}
	opts := options.UpdateOne().SetUpsert(true)
	if _, err := r.col.UpdateOne(ctx, filter, update, opts); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// RemovePlaylist removes a playlist link by ID ($pull, idempotent).
func (r *SocialFeaturesRepository) RemovePlaylist(ctx context.Context, eventoID, playlistID string) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$pull", Value: bson.D{
			{Key: "playlists", Value: bson.D{{Key: "id", Value: playlistID}}},
		}},
		{Key: "$set", Value: bson.D{{Key: "atualizadoEm", Value: now}}},
	}
	if _, err := r.col.UpdateOne(ctx, filter, update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// AddBringListItem appends an item to the event's bring list (upsert — lazy creates).
func (r *SocialFeaturesRepository) AddBringListItem(ctx context.Context, eventoID string, item social.BringListItem) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "bringList", Value: item}}},
		{Key: "$set", Value: bson.D{{Key: "atualizadoEm", Value: now}}},
		{Key: "$setOnInsert", Value: newSocialDefaults(eventoID, now)},
	}
	opts := options.UpdateOne().SetUpsert(true)
	if _, err := r.col.UpdateOne(ctx, filter, update, opts); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// UpdateBringListItem updates nome and quantidade of a bring list item via positional operator.
func (r *SocialFeaturesRepository) UpdateBringListItem(ctx context.Context, eventoID, itemID, nome, quantidade string) error {
	now := time.Now().UTC()
	// Match the document and the specific array element by id.
	filter := bson.D{
		{Key: "eventoId", Value: eventoID},
		{Key: "bringList.id", Value: itemID},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "bringList.$.nome", Value: nome},
			{Key: "bringList.$.quantidade", Value: quantidade},
			{Key: "atualizadoEm", Value: now},
		}},
	}
	if _, err := r.col.UpdateOne(ctx, filter, update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// RemoveBringListItem removes a bring list item by ID ($pull, idempotent).
func (r *SocialFeaturesRepository) RemoveBringListItem(ctx context.Context, eventoID, itemID string) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$pull", Value: bson.D{
			{Key: "bringList", Value: bson.D{{Key: "id", Value: itemID}}},
		}},
		{Key: "$set", Value: bson.D{{Key: "atualizadoEm", Value: now}}},
	}
	if _, err := r.col.UpdateOne(ctx, filter, update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// ClaimBringListItem atomically claims an unclaimed bring list item.
// Uses $elemMatch to match only the specific item when claimedBy is null/empty,
// then updates that element via the positional $ operator.
// Returns BRING_LIST_ITEM_ALREADY_CLAIMED conflict error when the item is already taken.
func (r *SocialFeaturesRepository) ClaimBringListItem(ctx context.Context, eventoID, itemID, usuarioID, usuarioNome string, claimedAt time.Time) error {
	now := time.Now().UTC()

	// Filter: event doc + array element where id matches AND claimedBy is absent/empty.
	// Using $elemMatch ensures the positional $ in the update targets the correct element.
	filter := bson.D{
		{Key: "eventoId", Value: eventoID},
		{Key: "bringList", Value: bson.D{
			{Key: "$elemMatch", Value: bson.D{
				{Key: "id", Value: itemID},
				{Key: "$or", Value: bson.A{
					bson.D{{Key: "claimedBy", Value: bson.D{{Key: "$exists", Value: false}}}},
					bson.D{{Key: "claimedBy", Value: nil}},
					bson.D{{Key: "claimedBy", Value: ""}},
				}},
			}},
		}},
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "bringList.$.claimedBy", Value: usuarioID},
			{Key: "bringList.$.claimedByNome", Value: usuarioNome},
			{Key: "bringList.$.claimedAt", Value: claimedAt},
			{Key: "atualizadoEm", Value: now},
		}},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var result socialFeaturesDocument
	if err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result); err != nil {
		if err == mongo.ErrNoDocuments {
			return apierr.Conflict("BRING_LIST_ITEM_ALREADY_CLAIMED")
		}
		return apierr.Internal(err.Error())
	}
	return nil
}

// UnclaimBringListItem clears the claim fields on a bring list item (idempotent).
func (r *SocialFeaturesRepository) UnclaimBringListItem(ctx context.Context, eventoID, itemID string) error {
	now := time.Now().UTC()
	filter := bson.D{
		{Key: "eventoId", Value: eventoID},
		{Key: "bringList.id", Value: itemID},
	}
	update := bson.D{
		{Key: "$unset", Value: bson.D{
			{Key: "bringList.$.claimedBy", Value: ""},
			{Key: "bringList.$.claimedByNome", Value: ""},
			{Key: "bringList.$.claimedAt", Value: ""},
		}},
		{Key: "$set", Value: bson.D{
			{Key: "atualizadoEm", Value: now},
		}},
	}
	// UpdateOne is idempotent: if item or document not found, MatchedCount == 0 is not an error.
	if _, err := r.col.UpdateOne(ctx, filter, update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// SetCheckinHabilitado sets the checkinHabilitado flag (upsert — lazy creates the document).
func (r *SocialFeaturesRepository) SetCheckinHabilitado(ctx context.Context, eventoID string, habilitado bool) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "checkinHabilitado", Value: habilitado},
			{Key: "atualizadoEm", Value: now},
		}},
		{Key: "$setOnInsert", Value: newSocialDefaults(eventoID, now)},
	}
	opts := options.UpdateOne().SetUpsert(true)
	if _, err := r.col.UpdateOne(ctx, filter, update, opts); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// AddCheckin atomically appends a check-in record, ensuring uniqueness per user.
// Uses "checkins.usuarioId $ne userID" filter to prevent duplicates.
// Returns CHECKIN_ALREADY_REGISTERED conflict error when the user has already checked in.
func (r *SocialFeaturesRepository) AddCheckin(ctx context.Context, eventoID string, c social.Checkin) error {
	now := time.Now().UTC()

	// Filter: event doc where this user has NOT already checked in.
	filter := bson.D{
		{Key: "eventoId", Value: eventoID},
		{Key: "checkins.usuarioId", Value: bson.D{{Key: "$ne", Value: c.UsuarioID}}},
	}
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "checkins", Value: c}}},
		{Key: "$set", Value: bson.D{{Key: "atualizadoEm", Value: now}}},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var result socialFeaturesDocument
	if err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result); err != nil {
		if err == mongo.ErrNoDocuments {
			return apierr.Conflict("CHECKIN_ALREADY_REGISTERED")
		}
		return apierr.Internal(err.Error())
	}
	return nil
}

// AddAlbumLink appends a photo album link to the event's album list (upsert — lazy creates).
func (r *SocialFeaturesRepository) AddAlbumLink(ctx context.Context, eventoID string, link social.AlbumLink) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$push", Value: bson.D{{Key: "albumLinks", Value: link}}},
		{Key: "$set", Value: bson.D{{Key: "atualizadoEm", Value: now}}},
		{Key: "$setOnInsert", Value: newSocialDefaults(eventoID, now)},
	}
	opts := options.UpdateOne().SetUpsert(true)
	if _, err := r.col.UpdateOne(ctx, filter, update, opts); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// RemoveAlbumLink removes a photo album link by ID ($pull, idempotent).
func (r *SocialFeaturesRepository) RemoveAlbumLink(ctx context.Context, eventoID, linkID string) error {
	now := time.Now().UTC()
	filter := bson.D{{Key: "eventoId", Value: eventoID}}
	update := bson.D{
		{Key: "$pull", Value: bson.D{
			{Key: "albumLinks", Value: bson.D{{Key: "id", Value: linkID}}},
		}},
		{Key: "$set", Value: bson.D{{Key: "atualizadoEm", Value: now}}},
	}
	if _, err := r.col.UpdateOne(ctx, filter, update); err != nil {
		return apierr.Internal(err.Error())
	}
	return nil
}

// ---- EventoAuthAdapter ----

// ---- EventoAuthAdapter ----

// EventoAuthAdapter implements out.EventoAuthPort by querying the eventos and
// participants collections — the same collections the Java backend uses.
type EventoAuthAdapter struct {
	eventoCol      *mongo.Collection
	participantCol *mongo.Collection
}

// NewEventoAuthAdapter creates a new EventoAuthAdapter.
func NewEventoAuthAdapter(client *Client) *EventoAuthAdapter {
	return &EventoAuthAdapter{
		eventoCol:      client.Collection(colEventos),
		participantCol: client.Collection("participants"),
	}
}

// FaseAtLeast validates that the event's stored status is at least the required minimum phase.
// For AGUARDANDO_ACEITE, only RASCUNHO is below the minimum.
func (a *EventoAuthAdapter) FaseAtLeast(ctx context.Context, eventoID string, fase social.EventoFase) error {
	filter := parseIDToFilter(eventoID)
	var doc struct {
		Status string `bson:"status"`
	}
	if err := a.eventoCol.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return apierr.NotFound("evento", eventoID)
		}
		return apierr.Internal(err.Error())
	}
	// Only RASCUNHO is strictly below AGUARDANDO_ACEITE in the Java phase ordering.
	if fase == social.FaseAguardandoAceite && doc.Status == "RASCUNHO" {
		return apierr.Unprocessable("evento não está na fase mínima requerida (AGUARDANDO_ACEITE)")
	}
	return nil
}

// IsOrganizador returns true when the given user is the owner of the event
// (checked via usuario_id_responsavel on the eventos collection).
func (a *EventoAuthAdapter) IsOrganizador(ctx context.Context, eventoID, usuarioID string) (bool, error) {
	filter := parseIDToFilter(eventoID)
	var doc struct {
		UsuarioIDResponsavel any `bson:"usuario_id_responsavel"`
	}
	if err := a.eventoCol.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return false, apierr.NotFound("evento", eventoID)
		}
		return false, apierr.Internal(err.Error())
	}
	return rawIDToString(doc.UsuarioIDResponsavel) == usuarioID, nil
}

// IsParticipanteConfirmadoOuOrganizador returns true when the user is the organizer
// or has a confirmed participant record (status ACEITO or CONFIRMADO) in the participants collection.
func (a *EventoAuthAdapter) IsParticipanteConfirmadoOuOrganizador(ctx context.Context, eventoID, usuarioID string) (bool, error) {
	isOrg, err := a.IsOrganizador(ctx, eventoID, usuarioID)
	if err != nil {
		return false, err
	}
	if isOrg {
		return true, nil
	}

	// participants.evento_id is stored as UUID binary (see participant.go).
	// participants.usuario_id uses userIDValue (ObjectID or UUID binary).
	// Java uses status=ACEITO; Go-created organizers use CONFIRMADO — accept both.
	filter := bson.D{
		{Key: "evento_id", Value: uuidStringToBinary(eventoID)},
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
		{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"ACEITO", "CONFIRMADO"}}}},
	}
	count, err := a.participantCol.CountDocuments(ctx, filter)
	if err != nil {
		return false, apierr.Internal(err.Error())
	}
	return count > 0, nil
}
