package mongodb

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// providerLoginDoc maps a ProviderLogin to MongoDB BSON.
type providerLoginDoc struct {
	Provider       string `bson:"provider"`
	ProviderUserID string `bson:"provider_user_id"`
	Nome           string `bson:"nome,omitempty"`
	Email          string `bson:"email,omitempty"`
	FotoPerfil     string `bson:"foto_perfil,omitempty"`
}

// telefoneDoc maps a Telefone to BSON.
type telefoneDoc struct {
	DDI    string `bson:"ddi,omitempty"`
	DDD    string `bson:"ddd,omitempty"`
	Numero string `bson:"numero,omitempty"`
	Tipo   string `bson:"tipo,omitempty"`
}

// enderecoDoc maps an Endereco to BSON.
type enderecoDoc struct {
	Rua         string `bson:"rua,omitempty"`
	Numero      string `bson:"numero,omitempty"`
	Complemento string `bson:"complemento,omitempty"`
	Bairro      string `bson:"bairro,omitempty"`
	Cidade      string `bson:"cidade,omitempty"`
	Estado      string `bson:"estado,omitempty"`
	CEP         string `bson:"cep,omitempty"`
}

// usuarioDocument is the MongoDB BSON document for Usuario.
// ID uses interface{} to support ObjectID (Go-created docs), UUID Binary (Java-created docs),
// and plain string _id values — enabling cross-compatibility between Java and Go backends.
type usuarioDocument struct {
	ID             interface{}        `bson:"_id,omitempty"`
	Nome           string             `bson:"nome"`
	Email          string             `bson:"email"`
	CPF            string             `bson:"cpf,omitempty"`
	SenhaHash      string             `bson:"senha_hash,omitempty"`
	DataNascimento *time.Time         `bson:"data_nascimento,omitempty"`
	FotoPerfil     string             `bson:"foto_perfil,omitempty"`
	Telefone       *telefoneDoc       `bson:"telefone,omitempty"`
	Endereco       *enderecoDoc       `bson:"endereco,omitempty"`
	ProviderLogin  []providerLoginDoc `bson:"provider_login,omitempty"`
	Roles          []string           `bson:"roles"`
	Ativo          bool               `bson:"ativo"`
	CriadoEm      time.Time          `bson:"criado_em"`
	UpdatedAt      time.Time          `bson:"updated_at"`
}

// userIDValue converts a user ID string to its appropriate BSON value for storage,
// preserving round-trip fidelity when read back via rawIDToString.
//
// Rules:
//   - 24-char hex → bson.ObjectID (for Go-created users whose _id is ObjectID)
//   - UUID string (8-4-4-4-12) → bson.Binary{Subtype: 4}
//   - Anything else → the string itself
//
// This is critical for ownership checks: rawIDToString(userIDValue(id)) == id.
func userIDValue(id string) interface{} {
	if id == "" {
		return nil
	}
	// Try as ObjectID hex (exactly 24 hex chars)
	if oid, err := bson.ObjectIDFromHex(id); err == nil {
		return oid
	}
	// Try as UUID string (8-4-4-4-12 format)
	if u, err := uuid.Parse(id); err == nil {
		b := [16]byte(u)
		return bson.Binary{Subtype: 0x04, Data: b[:]}
	}
	return id
}

// rawIDToString converts any MongoDB _id value to a string representation.
// Handles ObjectID (hex string), UUID Binary (UUID string format), and plain strings.
func rawIDToString(id interface{}) string {
	if id == nil {
		return ""
	}
	switch v := id.(type) {
	case bson.ObjectID:
		return v.Hex()
	case string:
		return v
	case bson.Binary:
		// UUID stored as BSON Binary (subtype 3 = old UUID, subtype 4 = UUID RFC 4122)
		if (v.Subtype == 3 || v.Subtype == 4) && len(v.Data) == 16 {
			b := v.Data
			return fmt.Sprintf("%s-%s-%s-%s-%s",
				hex.EncodeToString(b[0:4]),
				hex.EncodeToString(b[4:6]),
				hex.EncodeToString(b[6:8]),
				hex.EncodeToString(b[8:10]),
				hex.EncodeToString(b[10:16]))
		}
		return hex.EncodeToString(v.Data)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseIDToFilter builds a MongoDB _id filter from a string ID.
// Handles ObjectID hex strings, UUID strings (8-4-4-4-12 format), and plain strings.
func parseIDToFilter(id string) bson.D {
	// Try as ObjectID hex (exactly 24 hex chars)
	if oid, err := bson.ObjectIDFromHex(id); err == nil {
		return bson.D{{Key: "_id", Value: oid}}
	}
	// Try as UUID string (8-4-4-4-12 hex format → Binary subtype 4)
	parts := strings.Split(id, "-")
	if len(parts) == 5 {
		hexStr := strings.Join(parts, "")
		if b, err := hex.DecodeString(hexStr); err == nil && len(b) == 16 {
			return bson.D{{Key: "_id", Value: bson.Binary{Subtype: 0x04, Data: b}}}
		}
	}
	// Fallback: string _id
	return bson.D{{Key: "_id", Value: id}}
}

// UsuarioRepository implements portout.UsuarioRepository using MongoDB.
type UsuarioRepository struct {
	col *mongo.Collection
}

// NewUsuarioRepository creates a new UsuarioRepository.
func NewUsuarioRepository(client *Client) *UsuarioRepository {
	return &UsuarioRepository{col: client.Collection("usuarios")}
}

// FindByID returns a Usuario by its ID string (ObjectID hex, UUID string, or plain string).
func (r *UsuarioRepository) FindByID(ctx context.Context, id string) (*auth.Usuario, error) {
	filter := parseIDToFilter(id)
	var doc usuarioDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("usuario", id)
		}
		return nil, err
	}
	u := usuarioFromDoc(doc)
	return &u, nil
}

// FindByEmail returns a Usuario by email address.
func (r *UsuarioRepository) FindByEmail(ctx context.Context, email string) (*auth.Usuario, error) {
	var doc usuarioDocument
	if err := r.col.FindOne(ctx, bson.D{{Key: "email", Value: email}}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("usuario", email)
		}
		return nil, err
	}
	u := usuarioFromDoc(doc)
	return &u, nil
}

// FindByProviderID returns a Usuario linked to the given social provider.
func (r *UsuarioRepository) FindByProviderID(ctx context.Context, provider, providerUserID string) (*auth.Usuario, error) {
	filter := bson.D{
		{Key: "provider_login", Value: bson.D{
			{Key: "$elemMatch", Value: bson.D{
				{Key: "provider", Value: provider},
				{Key: "provider_user_id", Value: providerUserID},
			}},
		}},
	}
	var doc usuarioDocument
	if err := r.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, apierr.NotFound("usuario", provider+":"+providerUserID)
		}
		return nil, err
	}
	u := usuarioFromDoc(doc)
	return &u, nil
}

// Save inserts a new Usuario document with a freshly generated ObjectID.
func (r *UsuarioRepository) Save(ctx context.Context, u *auth.Usuario) (*auth.Usuario, error) {
	now := time.Now().UTC()
	doc := usuarioToDoc(u)
	doc.ID = bson.NewObjectID()
	doc.CriadoEm = now
	doc.UpdatedAt = now

	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	saved := usuarioFromDoc(doc)
	return &saved, nil
}

// Update updates an existing Usuario using $set to avoid replacing the _id field.
// This is required for cross-compatibility: Java may use UUID binary _id while Go uses ObjectID.
func (r *UsuarioRepository) Update(ctx context.Context, u *auth.Usuario) (*auth.Usuario, error) {
	filter := parseIDToFilter(u.ID)
	doc := usuarioToDoc(u)
	now := time.Now().UTC()

	setFields := bson.D{
		{Key: "nome", Value: doc.Nome},
		{Key: "email", Value: doc.Email},
		{Key: "ativo", Value: doc.Ativo},
		{Key: "roles", Value: doc.Roles},
		{Key: "updated_at", Value: now},
	}
	if doc.CPF != "" {
		setFields = append(setFields, bson.E{Key: "cpf", Value: doc.CPF})
	}
	if doc.SenhaHash != "" {
		setFields = append(setFields, bson.E{Key: "senha_hash", Value: doc.SenhaHash})
	}
	if doc.DataNascimento != nil {
		setFields = append(setFields, bson.E{Key: "data_nascimento", Value: doc.DataNascimento})
	}
	if doc.FotoPerfil != "" {
		setFields = append(setFields, bson.E{Key: "foto_perfil", Value: doc.FotoPerfil})
	}
	if doc.Telefone != nil {
		setFields = append(setFields, bson.E{Key: "telefone", Value: doc.Telefone})
	}
	if doc.Endereco != nil {
		setFields = append(setFields, bson.E{Key: "endereco", Value: doc.Endereco})
	}
	if len(doc.ProviderLogin) > 0 {
		setFields = append(setFields, bson.E{Key: "provider_login", Value: doc.ProviderLogin})
	}

	update := bson.D{{Key: "$set", Value: setFields}}
	if _, err := r.col.UpdateOne(ctx, filter, update); err != nil {
		return nil, err
	}

	// Fetch the updated document to return the current state
	var updated usuarioDocument
	if err := r.col.FindOne(ctx, filter).Decode(&updated); err != nil {
		return nil, err
	}
	result := usuarioFromDoc(updated)
	return &result, nil
}

// FindAll returns a paginated list of users.
func (r *UsuarioRepository) FindAll(ctx context.Context, page, pageSize int) ([]auth.Usuario, int64, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	skip := int64((page - 1) * pageSize)

	total, err := r.col.CountDocuments(ctx, bson.D{})
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "criado_em", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(pageSize))

	cur, err := r.col.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var docs []usuarioDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	result := make([]auth.Usuario, len(docs))
	for i, d := range docs {
		result[i] = usuarioFromDoc(d)
	}
	return result, total, nil
}

// DeleteByID removes a Usuario by ID string (ObjectID hex, UUID string, or plain string).
func (r *UsuarioRepository) DeleteByID(ctx context.Context, id string) error {
	filter := parseIDToFilter(id)
	res, err := r.col.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apierr.NotFound("usuario", id)
	}
	return nil
}

// ---- helpers ----

func usuarioFromDoc(doc usuarioDocument) auth.Usuario {
	roles := make([]auth.Role, len(doc.Roles))
	for i, r := range doc.Roles {
		roles[i] = auth.Role(r)
	}
	providers := make([]auth.ProviderLogin, len(doc.ProviderLogin))
	for i, p := range doc.ProviderLogin {
		providers[i] = auth.ProviderLogin{
			Provider:       p.Provider,
			ProviderUserID: p.ProviderUserID,
			Nome:           p.Nome,
			Email:          p.Email,
			FotoPerfil:     p.FotoPerfil,
		}
	}
	u := auth.Usuario{
		ID:            rawIDToString(doc.ID),
		Nome:          doc.Nome,
		Email:         doc.Email,
		CPF:           doc.CPF,
		SenhaHash:     doc.SenhaHash,
		DataNascimento: doc.DataNascimento,
		FotoPerfil:    doc.FotoPerfil,
		ProviderLogin: providers,
		Roles:         roles,
		Ativo:         doc.Ativo,
		CriadoEm:     doc.CriadoEm,
		UpdatedAt:     doc.UpdatedAt,
	}
	if doc.Telefone != nil {
		u.Telefone = &auth.Telefone{DDI: doc.Telefone.DDI, DDD: doc.Telefone.DDD, Numero: doc.Telefone.Numero, Tipo: doc.Telefone.Tipo}
	}
	if doc.Endereco != nil {
		u.Endereco = &auth.Endereco{
			Rua: doc.Endereco.Rua, Numero: doc.Endereco.Numero, Complemento: doc.Endereco.Complemento,
			Bairro: doc.Endereco.Bairro, Cidade: doc.Endereco.Cidade, Estado: doc.Endereco.Estado, CEP: doc.Endereco.CEP,
		}
	}
	return u
}

func usuarioToDoc(u *auth.Usuario) usuarioDocument {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}
	providers := make([]providerLoginDoc, len(u.ProviderLogin))
	for i, p := range u.ProviderLogin {
		providers[i] = providerLoginDoc{Provider: p.Provider, ProviderUserID: p.ProviderUserID, Nome: p.Nome, Email: p.Email, FotoPerfil: p.FotoPerfil}
	}
	doc := usuarioDocument{
		Nome:          u.Nome,
		Email:         u.Email,
		CPF:           u.CPF,
		SenhaHash:     u.SenhaHash,
		DataNascimento: u.DataNascimento,
		FotoPerfil:    u.FotoPerfil,
		ProviderLogin: providers,
		Roles:         roles,
		Ativo:         u.Ativo,
	}
	if u.Telefone != nil {
		doc.Telefone = &telefoneDoc{DDI: u.Telefone.DDI, DDD: u.Telefone.DDD, Numero: u.Telefone.Numero, Tipo: u.Telefone.Tipo}
	}
	if u.Endereco != nil {
		doc.Endereco = &enderecoDoc{
			Rua: u.Endereco.Rua, Numero: u.Endereco.Numero, Complemento: u.Endereco.Complemento,
			Bairro: u.Endereco.Bairro, Cidade: u.Endereco.Cidade, Estado: u.Endereco.Estado, CEP: u.Endereco.CEP,
		}
	}
	return doc
}


