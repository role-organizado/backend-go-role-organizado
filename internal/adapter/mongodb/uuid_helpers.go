package mongodb

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ---- Package-private ID helpers ----
// These are used internally by MongoDB adapter files for cross-Java/Go compatibility.
// Java stores entity IDs as UUID Binary (subtype 4); Go may use ObjectID or UUID strings.

// userIDValue converts a user ID string to its appropriate BSON value for storage,
// preserving round-trip fidelity: rawIDToString(userIDValue(id)) == id.
//
// Rules:
//   - 24-char hex → bson.ObjectID (Go-created users)
//   - UUID string (8-4-4-4-12) → bson.Binary{Subtype: 4} (Java-created users)
//   - Anything else → the string itself
func userIDValue(id string) interface{} {
	if id == "" {
		return nil
	}
	if oid, err := bson.ObjectIDFromHex(id); err == nil {
		return oid
	}
	if u, err := uuid.Parse(id); err == nil {
		b := [16]byte(u)
		return bson.Binary{Subtype: 0x04, Data: b[:]}
	}
	return id
}

// rawIDToString converts any MongoDB _id value to a canonical string representation.
// Handles: bson.ObjectID → hex; bson.Binary UUID → UUID string; string → as-is.
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
		// UUID stored as BSON Binary (subtype 3 = legacy UUID, subtype 4 = UUID RFC 4122)
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

// parseIDToFilter builds a MongoDB _id filter bson.D from a string ID.
// Tries ObjectID hex first, then UUID string, then falls back to plain string.
func parseIDToFilter(id string) bson.D {
	if oid, err := bson.ObjectIDFromHex(id); err == nil {
		return bson.D{{Key: "_id", Value: oid}}
	}
	parts := strings.Split(id, "-")
	if len(parts) == 5 {
		hexStr := strings.Join(parts, "")
		if b, err := hex.DecodeString(hexStr); err == nil && len(b) == 16 {
			return bson.D{{Key: "_id", Value: bson.Binary{Subtype: 0x04, Data: b}}}
		}
	}
	return bson.D{{Key: "_id", Value: id}}
}

// uuidStringToBinary is the unexported alias used by adapter files internally.
func uuidStringToBinary(s string) bson.Binary {
	return UUIDStringToBinary(s)
}

// uuidBinaryToString converts a bson.Binary (subtype 3 or 4) to a UUID string.
func uuidBinaryToString(b bson.Binary) string {
	if len(b.Data) != 16 {
		return ""
	}
	return uuid.UUID(b.Data).String()
}

// ---- Exported helpers (used by handler and usecase layers) ----

// BinaryToUUIDString converts a MongoDB Binary (subtype 4, UUID) to a UUID string.
// Java stores UUIDs as Binary subtype 4 (16 bytes); Go's bson.M decodes them as bson.Binary.
// Also accepts plain string values. Falls back to fmt.Sprintf for unknown types.
func BinaryToUUIDString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case bson.Binary:
		if len(val.Data) == 16 {
			b := val.Data
			return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
				b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
		}
	case string:
		return val
	}
	return fmt.Sprintf("%v", v)
}

// UUIDStringToBinary converts a UUID string (8-4-4-4-12) to a MongoDB Binary (subtype 4).
// Falls back to a newly generated random UUID if the input cannot be parsed.
func UUIDStringToBinary(s string) bson.Binary {
	u, err := uuid.Parse(s)
	if err != nil {
		u = uuid.New()
	}
	b := [16]byte(u)
	return bson.Binary{Subtype: 0x04, Data: b[:]}
}

// ToInt64 safely converts various numeric BSON types to int64.
// Returns 0 for nil or unrecognised types.
func ToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}

// GenerateID generates a new unique MongoDB ObjectID hex string for use as document IDs.
func GenerateID() string {
	return bson.NewObjectID().Hex()
}

// CamelToSnake converts a camelCase identifier to snake_case for MongoDB field names.
func CamelToSnake(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' && i > 0 {
			result = append(result, '_')
		}
		if c >= 'A' && c <= 'Z' {
			result = append(result, byte(c+32))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
