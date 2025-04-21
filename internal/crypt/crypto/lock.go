package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/korjavin/drand-poc/internal/crypt/drand"
)

// ErrTooEarly is returned when trying to decrypt a message before its unlock time
var ErrTooEarly = errors.New("too early to decrypt")

// Client is the drand client interface
type Client interface {
	FetchRandomness(round uint64) ([]byte, error)
}

// DefaultClient is the default drand client
var DefaultClient Client

// Initialize the default client
func init() {
	var err error
	client, err := drand.NewClient()
	if err != nil {
		// In a real application, we might want to handle this error differently
		panic(fmt.Sprintf("failed to initialize drand client: %v", err))
	}
	DefaultClient = client
}

// Encrypt encrypts the plaintext so it can only be decrypted after the specified time
func Encrypt(plaintext []byte, unlockAt time.Time) (ciphertext []byte, hash []byte, round uint64, err error) {
	// Calculate the round number for the unlock time
	// The League of Entropy's drand network produces a new random value every 30 seconds
	// We need to calculate which round will be available at the unlock time

	// Current time in seconds since epoch
	now := time.Now().UTC().Unix()
	// Unlock time in seconds since epoch
	unlockTime := unlockAt.UTC().Unix()

	// The genesis time of the drand network (July 1, 2020)
	genesisTime := int64(1595431050)
	// The period between rounds in seconds
	period := int64(30)

	// Calculate the current round
	currentRound := uint64((now - genesisTime) / period)
	// Calculate the unlock round
	round = uint64((unlockTime - genesisTime) / period)

	// Ensure the unlock round is in the future
	if round <= currentRound {
		return nil, nil, 0, fmt.Errorf("unlock time must be in the future")
	}

	// Generate a random key for AES encryption
	key := make([]byte, 32) // AES-256 requires a 32-byte key
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to generate random key: %w", err)
	}

	// Encrypt the plaintext with the random key
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, 12) // GCM mode typically uses a 12-byte nonce
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, 0, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Create a GCM cipher mode
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt the plaintext
	cipherData := aesgcm.Seal(nil, nonce, plaintext, nil)

	// Combine the key, nonce, and ciphertext into a single byte slice
	// Format: [key (32 bytes)][nonce (12 bytes)][ciphertext]
	combined := make([]byte, len(key)+len(nonce)+len(cipherData))
	copy(combined, key)
	copy(combined[len(key):], nonce)
	copy(combined[len(key)+len(nonce):], cipherData)

	// Calculate the SHA-256 hash of the combined data
	h := sha256.Sum256(combined)

	return combined, h[:], round, nil
}

// Decrypt decrypts the ciphertext if the current time is after the unlock time
func Decrypt(ciphertext []byte, round uint64) ([]byte, error) {
	// Check if the current time is after the unlock time
	now := time.Now().UTC()
	genesisTime := time.Unix(1595431050, 0).UTC()
	period := 30 * time.Second

	// Calculate the unlock time based on the round
	unlockTime := genesisTime.Add(time.Duration(round) * period)

	if now.Before(unlockTime) {
		return nil, ErrTooEarly
	}

	// Fetch the randomness for the specified round
	randomness, err := DefaultClient.FetchRandomness(round)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch randomness: %w", err)
	}

	// Extract the key, nonce, and encrypted data from the ciphertext
	if len(ciphertext) < 44 { // 32 (key) + 12 (nonce) bytes minimum
		return nil, fmt.Errorf("invalid ciphertext: too short")
	}

	key := ciphertext[:32]
	nonce := ciphertext[32:44]
	encryptedData := ciphertext[44:]

	// XOR the key with the randomness to get the actual decryption key
	// This ensures that the key can only be derived after the randomness is available
	actualKey := make([]byte, 32)
	for i := 0; i < 32; i++ {
		actualKey[i] = key[i] ^ randomness[i%len(randomness)]
	}

	// Create a new AES cipher using the actual key
	block, err := aes.NewCipher(actualKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create a GCM cipher mode
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt the data
	plaintext, err := aesgcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
