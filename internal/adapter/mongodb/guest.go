package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// guestDocument is the BSON representation of a Guest (collection: guests).
// Field names match the Java schema (snake_case + Spring Data MongoDB defaults).
type guestDocument struct {
	ID                     interface{} `bson:"_id,omitempty"`
	Nome                   string      `bson:"nome"`
	Telefone               string      `bson:"telefone,omitempty"`
	Email                  string      `bson:"email,omitempty"`
	CriadoEm               time.Time   `bson:"criado_em"`
	AtualizadoEm           time.Time   `bson:"atualizado_em"`
	EvoluidoParaUsuarioID  interface{} `bson:"evoluido_para_usuario_id,omitempty"`
	EvoluidoEm             *time.Time  `bson:"evoluido_em,omitempty"`
}

// GuestMongoRepository implements out.GuestRepository over the 'guests' collection.
type GuestMongoRepository struct {
	col *mongo.Collection
}

// NewGuestRepository builds a GuestMongoRepository.
func NewGuestRepository(client *Client) *GuestMongoRepository {
	return &GuestMongoRepository{col: client.Collection("guests")}
}

// Save inserts a new guest document with the domain's UUID as a Binary subtype-4 _id.
func (r *GuestMongoRepository) Save(ctx context.Context, g *domain.Guest) (*domain.Guest, error) {
	now := time.Now().UTC()
	doc := guestToDoc(g)
	doc.ID = UUIDStringToBinary(g.ID)
	if doc.CriadoEm.IsZero() {
		doc.CriadoEm = now
	}
	if doc.AtualizadoEm.IsZero() {
		doc.AtualizadoEm = now
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	saved := guestFromDoc(doc)
	return &saved, nil
}

// Update applies $set on mutable fields (nome, telefone, email, atualizado_em,
// evolucao). Does not replace _id (Java parity).
func (r *GuestMongoRepository) Update(ctx context.Context, g *domain.Guest) (*domain.Guest, error) {
	filter := parseIDToFilter(g.ID)
	now := time.Now().UTC()
	set := bson.D{
		{Key: "nome", Value: g.Nome},
		{Key: "telefone", Value: g.Telefone},
		{Key: "email", Value: g.Email},
		{Key: "atualizado_em", Value: now},
	}
	if g.EvoluidoParaUsuarioID != "" {
		set = append(set, bson.E{Key: "evoluido_para_usuario_id", Value: userIDValue(g.EvoluidoParaUsuarioID)})
	}
	if g.EvoluidoEm != nil {
		set = append(set, bson.E{Key: "evoluido_em", Value: g.EvoluidoEm})
	}
	if _, err := r.col.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: set}}); err != nil {
		return nil, err
	}
	var updated guestDocument
	if err := r.col.FindOne(ctx, filter).Decode(&updated); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("guest", g.ID)
		}
		return nil, err
	}
	out := guestFromDoc(updated)
	return &out, nil
}

// FindByID looks up the guest, returning 404 when missing.
func (r *GuestMongoRepository) FindByID(ctx context.Context, id string) (*domain.Guest, error) {
	var doc guestDocument
	if err := r.col.FindOne(ctx, parseIDToFilter(id)).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("guest", id)
		}
		return nil, err
	}
	g := guestFromDoc(doc)
	return &g, nil
}

// FindByTelefone returns the unique guest with matching telefone.
func (r *GuestMongoRepository) FindByTelefone(ctx context.Context, telefone string) (*domain.Guest, error) {
	if telefone == "" {
		return nil, apierr.NotFound("guest", "telefone:")
	}
	var doc guestDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "telefone", Value: telefone}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("guest", telefone)
		}
		return nil, err
	}
	g := guestFromDoc(doc)
	return &g, nil
}

// FindByEmail returns the unique guest with matching email.
func (r *GuestMongoRepository) FindByEmail(ctx context.Context, email string) (*domain.Guest, error) {
	if email == "" {
		return nil, apierr.NotFound("guest", "email:")
	}
	var doc guestDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "email", Value: email}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("guest", email)
		}
		return nil, err
	}
	g := guestFromDoc(doc)
	return &g, nil
}

// FindByTelefoneOrEmail prioritises telefone, then falls back to email. Returns
// nil + apierr.NotFound when neither matches — caller may treat the apierr.NotFound
// as "no existing record" via apierr.IsNotFound.
func (r *GuestMongoRepository) FindByTelefoneOrEmail(ctx context.Context, telefone, email string) (*domain.Guest, error) {
	if telefone != "" {
		if g, err := r.FindByTelefone(ctx, telefone); err == nil {
			return g, nil
		} else if !apierr.IsNotFound(err) {
			return nil, err
		}
	}
	if email != "" {
		return r.FindByEmail(ctx, email)
	}
	return nil, apierr.NotFound("guest", "")
}

// FindAll returns the first `limit` guests (default 100 via the Java hard cap).
func (r *GuestMongoRepository) FindAll(ctx context.Context, limit int) ([]domain.Guest, error) {
	if limit <= 0 {
		limit = 100
	}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.D{{Key: "criado_em", Value: -1}})
	cur, err := r.col.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []guestDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]domain.Guest, len(docs))
	for i, d := range docs {
		out[i] = guestFromDoc(d)
	}
	return out, nil
}

// FindAllByIDs returns guests matching any of the given IDs.
func (r *GuestMongoRepository) FindAllByIDs(ctx context.Context, ids []string) ([]domain.Guest, error) {
	if len(ids) == 0 {
		return []domain.Guest{}, nil
	}
	binIDs := make([]bson.Binary, 0, len(ids))
	for _, id := range ids {
		binIDs = append(binIDs, UUIDStringToBinary(id))
	}
	filter := bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: binIDs}}}}
	cur, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []guestDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]domain.Guest, len(docs))
	for i, d := range docs {
		out[i] = guestFromDoc(d)
	}
	return out, nil
}

// ExistsByTelefone reports whether any guest matches the telefone.
func (r *GuestMongoRepository) ExistsByTelefone(ctx context.Context, telefone string) (bool, error) {
	if telefone == "" {
		return false, nil
	}
	count, err := r.col.CountDocuments(ctx, bson.D{{Key: "telefone", Value: telefone}}, options.Count().SetLimit(1))
	return count > 0, err
}

// ExistsByEmail reports whether any guest matches the email.
func (r *GuestMongoRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, nil
	}
	count, err := r.col.CountDocuments(ctx, bson.D{{Key: "email", Value: email}}, options.Count().SetLimit(1))
	return count > 0, err
}

// ---- mapping ----

func guestFromDoc(doc guestDocument) domain.Guest {
	return domain.Guest{
		ID:                    rawIDToString(doc.ID),
		Nome:                  doc.Nome,
		Telefone:              doc.Telefone,
		Email:                 doc.Email,
		CriadoEm:              doc.CriadoEm,
		AtualizadoEm:          doc.AtualizadoEm,
		EvoluidoParaUsuarioID: rawIDToString(doc.EvoluidoParaUsuarioID),
		EvoluidoEm:            doc.EvoluidoEm,
	}
}

func guestToDoc(g *domain.Guest) guestDocument {
	doc := guestDocument{
		Nome:         g.Nome,
		Telefone:     g.Telefone,
		Email:        g.Email,
		CriadoEm:     g.CriadoEm,
		AtualizadoEm: g.AtualizadoEm,
		EvoluidoEm:   g.EvoluidoEm,
	}
	if g.EvoluidoParaUsuarioID != "" {
		doc.EvoluidoParaUsuarioID = userIDValue(g.EvoluidoParaUsuarioID)
	}
	return doc
}
