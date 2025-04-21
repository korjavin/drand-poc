package drand

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/drand/drand/chain"
	"github.com/drand/drand/client"
)

func TestFetchRandomness(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock response with a fixed randomness value
		mockResponse := `{
			"round": 1234,
			"randomness": "7b00000000000000000000000000000000000000000000000000000000000000",
			"signature": "mock-signature",
			"previous_signature": "mock-previous-signature"
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	// Create a test client that uses our mock server
	testClient := &Client{
		client: &mockDrandClient{},
	}

	// Test fetching randomness
	randomness, err := testClient.FetchRandomness(1234)
	if err != nil {
		t.Fatalf("Failed to fetch randomness: %v", err)
	}

	// Expected randomness (hex decoded from the mock response)
	expectedHex := "7b00000000000000000000000000000000000000000000000000000000000000"
	expected, err := hex.DecodeString(expectedHex)
	if err != nil {
		t.Fatalf("Failed to decode expected hex: %v", err)
	}

	// Compare the result
	if hex.EncodeToString(randomness) != expectedHex {
		t.Errorf("Unexpected randomness. Got: %x, Want: %x", randomness, expected)
	}
}

// mockDrandClient is a simple mock implementation of the drand client.Client interface
type mockDrandClient struct{}

// Get implements the client.Client interface
func (m *mockDrandClient) Get(ctx context.Context, round uint64) (client.Result, error) {
	// Create a mock response
	return &mockRandomness{
		randomness: []byte{0x7b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}, nil
}

// Watch implements the client.Client interface
func (m *mockDrandClient) Watch(ctx context.Context) <-chan client.Result {
	return nil
}

// Info implements the client.Client interface
func (m *mockDrandClient) Info(ctx context.Context) (*chain.Info, error) {
	return nil, nil
}

// RoundAt implements the client.Client interface
func (m *mockDrandClient) RoundAt(t time.Time) uint64 {
	return 0
}

// Close implements the io.Closer interface
func (m *mockDrandClient) Close() error {
	return nil
}

// mockRandomness implements the client.Result interface
type mockRandomness struct {
	randomness []byte
}

// Randomness returns the mock randomness
func (m *mockRandomness) Randomness() []byte {
	return m.randomness
}

// Round returns a mock round number
func (m *mockRandomness) Round() uint64 {
	return 1234
}

// Signature returns a mock signature
func (m *mockRandomness) Signature() []byte {
	return []byte("mock-signature")
}

// PreviousSignature returns a mock previous signature
func (m *mockRandomness) PreviousSignature() []byte {
	return []byte("mock-previous-signature")
}
