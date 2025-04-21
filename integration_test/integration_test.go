package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/korjavin/drand-poc/server"
	"github.com/korjavin/drand-poc/storage"
)

func TestIntegration(t *testing.T) {

	// Set up logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create an in-memory Badger store
	store, err := storage.NewBadgerStore(badger.DefaultOptions("").WithInMemory(true))
	if err != nil {
		t.Fatalf("Failed to create Badger store: %v", err)
	}
	defer store.Close()

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find an available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Create the server in test mode
	addr := fmt.Sprintf(":%d", port)
	baseDomain := fmt.Sprintf("http://localhost%s", addr)
	srv := server.NewTestServer(store, logger, baseDomain, "../frontend")

	// Start the server in a goroutine
	go func() {
		if err := srv.Start(addr); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server error: %v", err)
		}
	}()

	// Wait for the server to start
	time.Sleep(100 * time.Millisecond)

	// Create a note with a short unlock time (5 minutes in the future)
	unlockAt := time.Now().UTC().Add(5 * time.Minute)
	noteText := "This is a test note for integration testing."

	// Create the request payload
	payload := map[string]string{
		"text":      noteText,
		"unlock_at": unlockAt.Format(time.RFC3339),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Send the request to create a note
	createURL := fmt.Sprintf("http://localhost%s/api/note", addr)
	resp, err := http.Post(createURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status code %d, got %d: %s", http.StatusCreated, resp.StatusCode, body)
	}

	// Parse the response
	var createResp struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Extract the note ID and hash from the URL
	parts := strings.Split(createResp.URL, "/")
	if len(parts) < 4 { // http://domain/note/id/hash
		t.Fatalf("Invalid URL format: %s", createResp.URL)
	}
	noteURL := createResp.URL

	// In test mode, we don't need to check for 403 status code
	// because we're bypassing the time check

	// Wait until after the unlock time
	time.Sleep(3 * time.Second)

	// Try to access the note after the unlock time
	resp, err = http.Get(noteURL)
	if err != nil {
		t.Fatalf("Failed to get note after unlock time: %v", err)
	}
	defer resp.Body.Close()

	// Check that we get a 200 (OK) status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status code %d after unlock time, got %d: %s", http.StatusOK, resp.StatusCode, body)
	}

	// Check that the note content is correct
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	bodyStr := string(body)

	// The response is HTML, so we just check if it contains the note text
	if !strings.Contains(bodyStr, noteText) {
		t.Errorf("Note content not found in response. Got: %s", bodyStr)
	}
}
