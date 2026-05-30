package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// refreshTokenDocument is the MongoDB BSON document for RefreshToken.
type refreshTokenDocument struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	UsuarioID string        `bson:"usuario_id"`
	Token     string        `bson:"token"`
	ExpiresAt time.Time     `bson:"expires_at"`
	Used      bool          `bson:"used"`
	CreatedAt time.Time     `bson:"created_at"`
}

// RefreshTokenRepository implements portout.RefreshTokenRepository using MongoDB.
type RefreshTokenRepository struct {
	col *mongo.Collection
}

// NewRefreshTokenRepository creates a RefreshTokenRepository.
func NewRefreshTokenRepository(client *Client) *RefreshTokenRepository {
	return &RefreshTokenRepository{col: client.Collection("refresh_tokens")}
}

// Save inserts a new refresh token document.
func (r *RefreshTokenRepository) Save(ctx context.Context, rt *auth.RefreshToken) (*auth.RefreshToken, error) {
	doc := refreshTokenDocument{
		ID:        bson.NewObjectID(),
		UsuarioID: rt.UsuarioID,
		Token:     rt.Token,
		ExpiresAt: rt.ExpiresAt,
		Used:      false,
		CreatedAt: time.Now().UTC(),
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

// Revoke marks a refresh token as used.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "used", Value: true}}}}
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
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "used", Value: true}}}}
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: "usuario_id", Value: usuarioID}}, update)
	return err
}

func refreshTokenFromDoc(doc refreshTokenDocument) auth.RefreshToken {
	return auth.RefreshToken{
		ID:        doc.ID.Hex(),
		UsuarioID: doc.UsuarioID,
		Token:     doc.Token,
		ExpiresAt: doc.ExpiresAt,
		Used:      doc.Used,
		CreatedAt: doc.CreatedAt,
	}
}
