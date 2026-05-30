package storage

import "time"

// Arquivo represents a stored file in GridFS.
type Arquivo struct {
	ID          string
	OwnerID     string
	NomeOriginal string
	ContentType  string
	Tamanho     int64
	GridFSID    string
	BucketName  string
	CriadoEm   time.Time
}

// IsOwner returns true if userID owns this Arquivo.
func (a *Arquivo) IsOwner(userID string) bool {
	return a.OwnerID == userID
}
