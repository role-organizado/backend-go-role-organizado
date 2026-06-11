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

// ============================================================
// Biometric credentials (collection: biometric_credentials)
// ============================================================

type biometricCredentialDocument struct {
	ID             interface{} `bson:"_id,omitempty"`
	UsuarioID      interface{} `bson:"usuario_id"`
	DeviceID       string      `bson:"device_id"`
	PublicKey      string      `bson:"public_key"`
	DeviceName     string      `bson:"device_name,omitempty"`
	DeviceModel    string      `bson:"device_model,omitempty"`
	AndroidVersion string      `bson:"android_version,omitempty"`
	CreatedAt      time.Time   `bson:"created_at"`
	LastUsedAt     time.Time   `bson:"last_used_at,omitempty"`
	IsActive       bool        `bson:"is_active"`
	RevokedAt      *time.Time  `bson:"revoked_at,omitempty"`
	RevokedReason  string      `bson:"revoked_reason,omitempty"`
}

// BiometricCredentialMongoRepository implements out.BiometricCredentialRepository.
type BiometricCredentialMongoRepository struct {
	col *mongo.Collection
}

// NewBiometricCredentialRepository builds the credential repo.
func NewBiometricCredentialRepository(client *Client) *BiometricCredentialMongoRepository {
	return &BiometricCredentialMongoRepository{col: client.Collection("biometric_credentials")}
}

// Save inserts a new credential with the domain's UUID as a Binary subtype-4 _id.
func (r *BiometricCredentialMongoRepository) Save(ctx context.Context, c *domain.BiometricCredential) (*domain.BiometricCredential, error) {
	doc := credToDoc(c)
	doc.ID = UUIDStringToBinary(c.ID)
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now().UTC()
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	out := credFromDoc(doc)
	return &out, nil
}

// Update applies $set on mutable fields (LastUsedAt, IsActive, RevokedAt, RevokedReason).
func (r *BiometricCredentialMongoRepository) Update(ctx context.Context, c *domain.BiometricCredential) (*domain.BiometricCredential, error) {
	filter := parseIDToFilter(c.ID)
	set := bson.D{
		{Key: "last_used_at", Value: c.LastUsedAt},
		{Key: "is_active", Value: c.IsActive},
	}
	if c.RevokedAt != nil {
		set = append(set, bson.E{Key: "revoked_at", Value: c.RevokedAt})
	}
	if c.RevokedReason != "" {
		set = append(set, bson.E{Key: "revoked_reason", Value: c.RevokedReason})
	}
	if _, err := r.col.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: set}}); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, c.ID)
}

// FindByID returns the credential, 404 when missing.
func (r *BiometricCredentialMongoRepository) FindByID(ctx context.Context, id string) (*domain.BiometricCredential, error) {
	var doc biometricCredentialDocument
	if err := r.col.FindOne(ctx, parseIDToFilter(id)).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("biometric_credential", id)
		}
		return nil, err
	}
	c := credFromDoc(doc)
	return &c, nil
}

// FindByDeviceID returns the first credential matching the deviceID (any state).
func (r *BiometricCredentialMongoRepository) FindByDeviceID(ctx context.Context, deviceID string) (*domain.BiometricCredential, error) {
	var doc biometricCredentialDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "device_id", Value: deviceID}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("biometric_credential", deviceID)
		}
		return nil, err
	}
	c := credFromDoc(doc)
	return &c, nil
}

// FindByUsuarioIDAndDeviceID returns the credential for the (user, device) pair (any state).
func (r *BiometricCredentialMongoRepository) FindByUsuarioIDAndDeviceID(ctx context.Context, usuarioID, deviceID string) (*domain.BiometricCredential, error) {
	filter := bson.D{
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
		{Key: "device_id", Value: deviceID},
	}
	var doc biometricCredentialDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("biometric_credential", usuarioID+":"+deviceID)
		}
		return nil, err
	}
	c := credFromDoc(doc)
	return &c, nil
}

// FindByUsuarioIDAndDeviceIDActive returns the ACTIVE credential for the (user, device).
func (r *BiometricCredentialMongoRepository) FindByUsuarioIDAndDeviceIDActive(ctx context.Context, usuarioID, deviceID string) (*domain.BiometricCredential, error) {
	filter := bson.D{
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
		{Key: "device_id", Value: deviceID},
		{Key: "is_active", Value: true},
	}
	var doc biometricCredentialDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("biometric_credential", usuarioID+":"+deviceID)
		}
		return nil, err
	}
	c := credFromDoc(doc)
	return &c, nil
}

// FindByUsuarioIDActive returns all ACTIVE credentials for a user.
func (r *BiometricCredentialMongoRepository) FindByUsuarioIDActive(ctx context.Context, usuarioID string) ([]domain.BiometricCredential, error) {
	filter := bson.D{
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
		{Key: "is_active", Value: true},
	}
	return r.findMany(ctx, filter)
}

// FindByUsuarioID returns all credentials for a user (any state).
func (r *BiometricCredentialMongoRepository) FindByUsuarioID(ctx context.Context, usuarioID string) ([]domain.BiometricCredential, error) {
	filter := bson.D{{Key: "usuario_id", Value: userIDValue(usuarioID)}}
	return r.findMany(ctx, filter)
}

// ExistsByDeviceIDActive reports whether any active credential exists for the deviceID.
func (r *BiometricCredentialMongoRepository) ExistsByDeviceIDActive(ctx context.Context, deviceID string) (bool, error) {
	count, err := r.col.CountDocuments(
		ctx,
		bson.D{{Key: "device_id", Value: deviceID}, {Key: "is_active", Value: true}},
		options.Count().SetLimit(1),
	)
	return count > 0, err
}

// DeleteByID removes a credential by ID (404 when missing).
func (r *BiometricCredentialMongoRepository) DeleteByID(ctx context.Context, id string) error {
	res, err := r.col.DeleteOne(ctx, parseIDToFilter(id))
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("biometric_credential", id)
	}
	return nil
}

// DeleteByUsuarioID removes all credentials for a user.
func (r *BiometricCredentialMongoRepository) DeleteByUsuarioID(ctx context.Context, usuarioID string) error {
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userIDValue(usuarioID)}})
	return err
}

// CountByUsuarioIDActive returns the count of active credentials for a user.
func (r *BiometricCredentialMongoRepository) CountByUsuarioIDActive(ctx context.Context, usuarioID string) (int64, error) {
	return r.col.CountDocuments(ctx, bson.D{
		{Key: "usuario_id", Value: userIDValue(usuarioID)},
		{Key: "is_active", Value: true},
	})
}

func (r *BiometricCredentialMongoRepository) findMany(ctx context.Context, filter bson.D) ([]domain.BiometricCredential, error) {
	cur, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []biometricCredentialDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]domain.BiometricCredential, len(docs))
	for i, d := range docs {
		out[i] = credFromDoc(d)
	}
	return out, nil
}

func credFromDoc(doc biometricCredentialDocument) domain.BiometricCredential {
	return domain.BiometricCredential{
		ID:             rawIDToString(doc.ID),
		UsuarioID:      rawIDToString(doc.UsuarioID),
		DeviceID:       doc.DeviceID,
		PublicKey:      doc.PublicKey,
		DeviceName:     doc.DeviceName,
		DeviceModel:    doc.DeviceModel,
		AndroidVersion: doc.AndroidVersion,
		CreatedAt:      doc.CreatedAt,
		LastUsedAt:     doc.LastUsedAt,
		IsActive:       doc.IsActive,
		RevokedAt:      doc.RevokedAt,
		RevokedReason:  doc.RevokedReason,
	}
}

func credToDoc(c *domain.BiometricCredential) biometricCredentialDocument {
	doc := biometricCredentialDocument{
		UsuarioID:      userIDValue(c.UsuarioID),
		DeviceID:       c.DeviceID,
		PublicKey:      c.PublicKey,
		DeviceName:     c.DeviceName,
		DeviceModel:    c.DeviceModel,
		AndroidVersion: c.AndroidVersion,
		CreatedAt:      c.CreatedAt,
		LastUsedAt:     c.LastUsedAt,
		IsActive:       c.IsActive,
		RevokedAt:      c.RevokedAt,
		RevokedReason:  c.RevokedReason,
	}
	return doc
}

// ============================================================
// Biometric challenges (collection: biometric_challenges)
// ============================================================

type biometricChallengeDocument struct {
	ID         interface{} `bson:"_id,omitempty"`
	DeviceID   string      `bson:"device_id"`
	Challenge  string      `bson:"challenge"`
	CreatedAt  time.Time   `bson:"created_at"`
	ExpiresAt  time.Time   `bson:"expires_at"`
	Used       bool        `bson:"used"`
	UsedAt     *time.Time  `bson:"used_at,omitempty"`
	UsedFromIP string      `bson:"used_from_ip,omitempty"`
}

// BiometricChallengeMongoRepository implements out.BiometricChallengeRepository.
type BiometricChallengeMongoRepository struct {
	col *mongo.Collection
}

// NewBiometricChallengeRepository builds the challenge repo.
func NewBiometricChallengeRepository(client *Client) *BiometricChallengeMongoRepository {
	return &BiometricChallengeMongoRepository{col: client.Collection("biometric_challenges")}
}

// Save inserts a new challenge.
func (r *BiometricChallengeMongoRepository) Save(ctx context.Context, c *domain.BiometricChallenge) (*domain.BiometricChallenge, error) {
	doc := challengeToDoc(c)
	doc.ID = UUIDStringToBinary(c.ID)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	out := challengeFromDoc(doc)
	return &out, nil
}

// Update flips Used/UsedAt/UsedFromIP (the only mutable fields).
func (r *BiometricChallengeMongoRepository) Update(ctx context.Context, c *domain.BiometricChallenge) (*domain.BiometricChallenge, error) {
	filter := parseIDToFilter(c.ID)
	set := bson.D{
		{Key: "used", Value: c.Used},
	}
	if c.UsedAt != nil {
		set = append(set, bson.E{Key: "used_at", Value: c.UsedAt})
	}
	if c.UsedFromIP != "" {
		set = append(set, bson.E{Key: "used_from_ip", Value: c.UsedFromIP})
	}
	if _, err := r.col.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: set}}); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, c.ID)
}

// FindByID returns the challenge, 404 when missing.
func (r *BiometricChallengeMongoRepository) FindByID(ctx context.Context, id string) (*domain.BiometricChallenge, error) {
	var doc biometricChallengeDocument
	if err := r.col.FindOne(ctx, parseIDToFilter(id)).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("biometric_challenge", id)
		}
		return nil, err
	}
	c := challengeFromDoc(doc)
	return &c, nil
}

// FindLatestByDeviceIDUnused returns the most-recent unused challenge for the deviceID.
func (r *BiometricChallengeMongoRepository) FindLatestByDeviceIDUnused(ctx context.Context, deviceID string) (*domain.BiometricChallenge, error) {
	filter := bson.D{
		{Key: "device_id", Value: deviceID},
		{Key: "used", Value: false},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	var doc biometricChallengeDocument
	if err := r.col.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("biometric_challenge", deviceID)
		}
		return nil, err
	}
	c := challengeFromDoc(doc)
	return &c, nil
}

// DeleteByDeviceID purges all challenges for the deviceID (used when issuing a new
// challenge or revoking the device).
func (r *BiometricChallengeMongoRepository) DeleteByDeviceID(ctx context.Context, deviceID string) error {
	_, err := r.col.DeleteMany(ctx, bson.D{{Key: "device_id", Value: deviceID}})
	return err
}

// DeleteByID removes a single challenge.
func (r *BiometricChallengeMongoRepository) DeleteByID(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, parseIDToFilter(id))
	return err
}

func challengeFromDoc(doc biometricChallengeDocument) domain.BiometricChallenge {
	return domain.BiometricChallenge{
		ID:         rawIDToString(doc.ID),
		DeviceID:   doc.DeviceID,
		Challenge:  doc.Challenge,
		CreatedAt:  doc.CreatedAt,
		ExpiresAt:  doc.ExpiresAt,
		Used:       doc.Used,
		UsedAt:     doc.UsedAt,
		UsedFromIP: doc.UsedFromIP,
	}
}

func challengeToDoc(c *domain.BiometricChallenge) biometricChallengeDocument {
	return biometricChallengeDocument{
		DeviceID:   c.DeviceID,
		Challenge:  c.Challenge,
		CreatedAt:  c.CreatedAt,
		ExpiresAt:  c.ExpiresAt,
		Used:       c.Used,
		UsedAt:     c.UsedAt,
		UsedFromIP: c.UsedFromIP,
	}
}
