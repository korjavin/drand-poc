package storage

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrNotFound = errors.New("note not found")
)

// Note represents a stored encrypted note
type Note struct {
	ID       string    // UUIDv4
	Hash     string    // hex(sha256(cipher))
	Cipher   []byte    // Encrypted data
	Round    uint64    // drand round number
	UnlockAt time.Time // Time when the note can be decrypted
}

// Store defines the interface for storing and retrieving notes
type Store interface {
	// Save stores a note in the database
	Save(ctx context.Context, n Note) error
	
	// Get retrieves a note by its ID and hash
	Get(ctx context.Context, id, hash string) (Note, error)
}
