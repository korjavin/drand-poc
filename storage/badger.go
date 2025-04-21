package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v3"
)

// BadgerStore implements the Store interface using Badger DB
type BadgerStore struct {
	db *badger.DB
}

// NewBadgerStore creates a new BadgerStore with the given options
func NewBadgerStore(opts badger.Options) (*BadgerStore, error) {
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}
	return &BadgerStore{db: db}, nil
}

// Close closes the underlying Badger database
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// Save stores a note in the database with TTL
func (s *BadgerStore) Save(ctx context.Context, n Note) error {
	// Calculate TTL: UnlockAt + 7 days
	ttl := time.Until(n.UnlockAt.Add(7 * 24 * time.Hour))

	// Marshal the note to JSON
	data, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("failed to marshal note: %w", err)
	}

	// Create a composite key: id:hash
	key := []byte(fmt.Sprintf("%s:%s", n.ID, n.Hash))

	// Store the note in the database with TTL
	err = s.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry(key, data).WithTTL(ttl)
		return txn.SetEntry(entry)
	})

	if err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	return nil
}

// Get retrieves a note by its ID and hash
func (s *BadgerStore) Get(ctx context.Context, id, hash string) (Note, error) {
	var note Note

	// Create the composite key
	key := []byte(fmt.Sprintf("%s:%s", id, hash))

	// Retrieve the note from the database
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return ErrNotFound
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &note)
		})
	})

	if err != nil {
		if err == ErrNotFound {
			return Note{}, ErrNotFound
		}
		return Note{}, fmt.Errorf("failed to get note: %w", err)
	}

	return note, nil
}
