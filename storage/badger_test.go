package storage

import (
	"context"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/google/uuid"
)

func TestBadgerStore(t *testing.T) {
	// Create an in-memory Badger database
	opts := badger.DefaultOptions("").WithInMemory(true)
	store, err := NewBadgerStore(opts)
	if err != nil {
		t.Fatalf("Failed to create BadgerStore: %v", err)
	}
	defer store.Close()
	
	// Create a test note
	id := uuid.New().String()
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cipher := []byte("encrypted data")
	round := uint64(12345)
	unlockAt := time.Now().Add(1 * time.Hour)
	
	note := Note{
		ID:       id,
		Hash:     hash,
		Cipher:   cipher,
		Round:    round,
		UnlockAt: unlockAt,
	}
	
	// Save the note
	ctx := context.Background()
	err = store.Save(ctx, note)
	if err != nil {
		t.Fatalf("Failed to save note: %v", err)
	}
	
	// Retrieve the note
	retrieved, err := store.Get(ctx, id, hash)
	if err != nil {
		t.Fatalf("Failed to get note: %v", err)
	}
	
	// Verify the retrieved note
	if retrieved.ID != id {
		t.Errorf("Expected ID %s, got %s", id, retrieved.ID)
	}
	if retrieved.Hash != hash {
		t.Errorf("Expected Hash %s, got %s", hash, retrieved.Hash)
	}
	if string(retrieved.Cipher) != string(cipher) {
		t.Errorf("Expected Cipher %s, got %s", cipher, retrieved.Cipher)
	}
	if retrieved.Round != round {
		t.Errorf("Expected Round %d, got %d", round, retrieved.Round)
	}
	
	// Test retrieving a non-existent note
	_, err = store.Get(ctx, "non-existent", "hash")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}
