package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"testing"
	"time"
)

// mockClient is a mock implementation of the Client interface for testing
type mockClient struct {
	randomness []byte
}

func (m *mockClient) FetchRandomness(round uint64) ([]byte, error) {
	return m.randomness, nil
}

func TestSimpleEncryptDecrypt(t *testing.T) {
	// Create a simple encryption/decryption test without using the drand client
	plaintext := []byte("This is a secret message")

	// Generate a key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Create a cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("Failed to create cipher block: %v", err)
	}

	// Create GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("Failed to create GCM: %v", err)
	}

	// Create a nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}

	// Encrypt
	ciphertext := aesGCM.Seal(nil, nonce, plaintext, nil)

	// Decrypt
	decrypted, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	// Verify
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted text doesn't match original. Got: %s, Want: %s", decrypted, plaintext)
	}
}

func TestEncryptWithDrand(t *testing.T) {
	// Save the original client and restore it after the test
	originalClient := DefaultClient
	defer func() { DefaultClient = originalClient }()

	// Create a mock client with fixed randomness
	mockRandomness := make([]byte, 32)
	for i := range mockRandomness {
		mockRandomness[i] = byte(i)
	}
	DefaultClient = &mockClient{randomness: mockRandomness}

	// Test data
	plaintext := []byte("This is a secret message")

	// Encrypt with a future time
	unlockAt := time.Now().UTC().Add(10 * time.Minute)
	_, hash, round, err := Encrypt(plaintext, unlockAt)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Verify the hash
	if len(hash) != 32 {
		t.Errorf("Expected hash length to be 32, got %d", len(hash))
	}

	// Verify the round is in the future
	currentTime := time.Now().UTC()
	genesisTime := time.Unix(1595431050, 0).UTC()
	period := 30 * time.Second
	currentRound := uint64(currentTime.Sub(genesisTime) / period)

	if round <= currentRound {
		t.Errorf("Expected round to be in the future. Got: %d, Current: %d", round, currentRound)
	}
}

func TestDecryptTooEarly(t *testing.T) {
	// Save the original client and restore it after the test
	originalClient := DefaultClient
	defer func() { DefaultClient = originalClient }()

	// Create a mock client with fixed randomness
	mockRandomness := make([]byte, 32)
	for i := range mockRandomness {
		mockRandomness[i] = byte(i)
	}
	DefaultClient = &mockClient{randomness: mockRandomness}

	// Test data
	plaintext := []byte("This is a secret message")

	// Encrypt with a future time
	unlockAt := time.Now().UTC().Add(10 * time.Minute) // Far in the future
	cipher, _, round, err := Encrypt(plaintext, unlockAt)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Try to decrypt before the unlock time
	_, err = Decrypt(cipher, round)
	if err != ErrTooEarly {
		t.Errorf("Expected ErrTooEarly, got: %v", err)
	}
}
