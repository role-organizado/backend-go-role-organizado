package mongodb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// refreshTokenDocument is the MongoDB BSON document for RefreshToken.
// Schema matches Java's Spring Data MongoDB storage:
//   - _id: binData (UUID binary subtype 4)
//   - usuario_id: binData (UUID binary subtype 4)
//   - criado_em: date (NOT created_at)
//   - usado_em: date|null (marks when used, null when active)
type refreshTokenDocument struct {
	ID        bson.Binary `bson:"_id"`
	UsuarioID bson.Binary `bson:"usuario_id"`
	Token     string      `bson:"token"`
	ExpiresAt time.Time   `bson:"expires_at"`
	CriadoEm  time.Time   `bson:"criado_em"`
	UsadoEm   *time.Time  `bson:"usado_em,omitempty"`
}

// uuidStringToBinary parses a UUID string (8-4-4-4-12) into a bson.Binary subtype 4.
// Falls back to random UUID if parsing fails.
func uuidStringToBinary(s string) bson.Binary {
	u, err := uuid.Parse(s)
	if err != nil {
		u = uuid.New()
	}
	b := [16]byte(u)
	return bson.Binary{Subtype: 0x04, Data: b[:]}
}

// uuidBinaryToString converts a bson.Binary (subtype 3 or 4) to a UUID string.
func uuidBinaryToString(b bson.Binary) string {
	if len(b.Data) != 16 {
		return ""
	}
	return uuid.UUID(b.Data).String()
}

// RefreshTokenRepository implements portout.RefreshTokenRepository using MongoDB.
type RefreshTokenRepository struct {
	col *mongo.Collection
}

// NewRefreshTokenRepository creates a RefreshTokenRepository.
func NewRefreshTokenRepository(client *Client) *RefreshTokenRepository {
	return &RefreshTokenRepository{col: client.Collection("refresh_tokens")}
}

// Save inserts a new refresh token document using UUID binary IDs (Java-compatible schema).
func (r *RefreshTokenRepository) Save(ctx context.Context, rt *auth.RefreshToken) (*auth.RefreshToken, error) {
	newID := uuid.New()
	idBytes := [16]byte(newID)
	doc := refreshTokenDocument{
		ID:        bson.Binary{Subtype: 0x04, Data: idBytes[:]},
		UsuarioID: uuidStringToBinary(rt.UsuarioID),
		Token:     rt.Token,
		ExpiresAt: rt.ExpiresAt,
		CriadoEm:  time.Now().UTC(),
		UsadoEm:   nil,
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	saved := refreshTokenFromDoc(doc)
	return &saved, nil
}

// FindByToken returns a refresh token by its token string.
func (r *RefreshTokenRepository) FindByToken(ctx context.Context, token string) (*auth.RefreshToken, error) {
	var doc refreshTokenDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "token", Value: token}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("refresh_token", token)
		}
		return nil, err
	}
	rt := refreshTokenFromDoc(doc)
	return &rt, nil
}

// Revoke marks a refresh token as used by setting usado_em to now.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	now := time.Now().UTC()
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "usado_em", Value: now}}}}
	res, err := r.col.UpdateOne(ctx, bson.D{{Key: "token", Value: token}}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return apierr.NotFound("refresh_token", token)
	}
	return nil
}

// RevokeAllForUser marks all refresh tokens for a user as used.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, usuarioID string) error {
	now := time.Now().UTC()
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "usado_em", Value: now}}}}
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: "usuario_id", Value: uuidStringToBinary(usuarioID)}}, update)
	return err
}

func refreshTokenFromDoc(doc refreshTokenDocument) auth.RefreshToken {
	used := doc.UsadoEm != nil
	var createdAt time.Time
	createdAt = doc.CriadoEm
	return auth.RefreshToken{
		ID:        uuidBinaryToString(doc.ID),
		UsuarioID: uuidBinaryToString(doc.UsuarioID),
		Token:     doc.Token,
		ExpiresAt: doc.ExpiresAt,
		Used:      used,
		CreatedAt: createdAt,
	}
}
