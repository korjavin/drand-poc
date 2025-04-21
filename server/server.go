package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/korjavin/drand-poc/internal/crypt/crypto"
	"github.com/korjavin/drand-poc/storage"
)

// Server represents the HTTP server
type Server struct {
	store      storage.Store
	logger     *slog.Logger
	baseDomain string
	staticDir  string
	testMode   bool // Used for testing to bypass time checks
}

// NewServer creates a new HTTP server
func NewServer(store storage.Store, logger *slog.Logger, baseDomain, staticDir string) *Server {
	return &Server{
		store:      store,
		logger:     logger,
		baseDomain: baseDomain,
		staticDir:  staticDir,
		testMode:   false,
	}
}

// NewTestServer creates a new HTTP server in test mode
func NewTestServer(store storage.Store, logger *slog.Logger, baseDomain, staticDir string) *Server {
	return &Server{
		store:      store,
		logger:     logger,
		baseDomain: baseDomain,
		staticDir:  staticDir,
		testMode:   true,
	}
}

// CreateNoteRequest represents the request body for creating a new note
type CreateNoteRequest struct {
	Text     string `json:"text"`
	UnlockAt string `json:"unlock_at"` // RFC3339 format
}

// CreateNoteResponse represents the response body for creating a new note
type CreateNoteResponse struct {
	URL string `json:"url"`
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/note", s.handleCreateNote)

	// Static routes
	mux.HandleFunc("GET /note/{id}/{h}", s.handleGetNote)
	mux.HandleFunc("GET /", s.handleIndex)

	// Static files
	fs := http.FileServer(http.Dir(s.staticDir))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	s.logger.Info("Starting server", "addr", addr)
	return http.ListenAndServe(addr, s.loggingMiddleware(mux))
}

// loggingMiddleware logs all HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		r = r.WithContext(ctx)

		start := time.Now()
		s.logger.Info("Request started",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		next.ServeHTTP(w, r)

		duration := time.Since(start)
		s.logger.Info("Request completed",
			"request_id", requestID,
			"duration", duration,
		)
	})
}

// handleCreateNote handles the POST /api/note endpoint
func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	requestID := r.Context().Value("request_id").(string)
	logger := s.logger.With("request_id", requestID)

	// Parse the request body
	var req CreateNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode request body", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if req.Text == "" {
		logger.Error("Empty text in request")
		http.Error(w, "Text cannot be empty", http.StatusBadRequest)
		return
	}

	// Parse the unlock time
	unlockAt, err := time.Parse(time.RFC3339, req.UnlockAt)
	if err != nil {
		logger.Error("Invalid unlock_at format", "error", err)
		http.Error(w, "Invalid unlock_at format. Use RFC3339 format (e.g., 2023-01-01T12:00:00Z)", http.StatusBadRequest)
		return
	}

	// Encrypt the note
	var cipher []byte
	var hash []byte
	var round uint64

	if s.testMode {
		// In test mode, store the plaintext directly
		cipher = []byte(req.Text)

		// Generate a fake hash
		hash = make([]byte, 32)
		for i := range hash {
			hash[i] = byte(i)
		}

		// Use a fake round number
		round = 12345
	} else {
		// In normal mode, encrypt the note
		var encryptErr error
		cipher, hash, round, encryptErr = crypto.Encrypt([]byte(req.Text), unlockAt)
		if encryptErr != nil {
			logger.Error("Failed to encrypt note", "error", encryptErr)
			http.Error(w, "Failed to encrypt note", http.StatusInternalServerError)
			return
		}
	}

	// Generate a UUID for the note
	id := uuid.New().String()

	// Convert the hash to a hex string
	hashHex := hex.EncodeToString(hash)

	// Create a note
	note := storage.Note{
		ID:       id,
		Hash:     hashHex,
		Cipher:   cipher,
		Round:    round,
		UnlockAt: unlockAt,
	}

	// Save the note
	if err := s.store.Save(r.Context(), note); err != nil {
		logger.Error("Failed to save note", "error", err)
		http.Error(w, "Failed to save note", http.StatusInternalServerError)
		return
	}

	// Generate the URL
	url := fmt.Sprintf("%s/note/%s/%s", s.baseDomain, id, hashHex)

	// Return the URL
	resp := CreateNoteResponse{URL: url}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Error("Failed to encode response", "error", err)
	}
}

// handleGetNote handles the GET /{id}/{h} endpoint
func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	requestID := r.Context().Value("request_id").(string)
	logger := s.logger.With("request_id", requestID)

	// Extract the ID and hash from the URL
	id := r.PathValue("id")
	hash := r.PathValue("h")

	// Get the note from the store
	note, err := s.store.Get(r.Context(), id, hash)
	if err != nil {
		if err == storage.ErrNotFound {
			logger.Info("Note not found", "id", id, "hash", hash)
			http.Error(w, "Note not found", http.StatusNotFound)
		} else {
			logger.Error("Failed to get note", "error", err, "id", id, "hash", hash)
			http.Error(w, "Failed to get note", http.StatusInternalServerError)
		}
		return
	}

	// Try to decrypt the note
	var plaintext []byte
	var decryptErr error

	if s.testMode {
		// In test mode, bypass the time check
		plaintext = note.Cipher // In test mode, we store the plaintext directly
		decryptErr = nil
	} else {
		// In normal mode, decrypt the note
		plaintext, decryptErr = crypto.Decrypt(note.Cipher, note.Round)
	}

	if decryptErr != nil {
		if decryptErr == crypto.ErrTooEarly {
			logger.Info("Too early to decrypt note", "id", id, "hash", hash, "unlock_at", note.UnlockAt)

			// Calculate the remaining time
			remaining := note.UnlockAt.Sub(time.Now())

			// Render the "too early" template
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)

			tmpl := template.Must(template.New("too_early").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>Note Locked</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/water.css@2/out/water.css">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body>
    <h1>Note Locked</h1>
    <p>This note is locked until {{.UnlockAt}}.</p>
    <p>Remaining time: {{.Remaining}}</p>
</body>
</html>
`))

			data := struct {
				UnlockAt  string
				Remaining string
			}{
				UnlockAt:  note.UnlockAt.Format(time.RFC1123),
				Remaining: remaining.Round(time.Second).String(),
			}

			if err := tmpl.Execute(w, data); err != nil {
				logger.Error("Failed to render template", "error", err)
			}
			return
		}

		logger.Error("Failed to decrypt note", "error", decryptErr, "id", id, "hash", hash)
		http.Error(w, "Failed to decrypt note", http.StatusInternalServerError)
		return
	}

	// Render the note
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	tmpl := template.Must(template.New("note").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>Decrypted Note</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/water.css@2/out/water.css">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body>
    <h1>Decrypted Note</h1>
    <pre>{{.Content}}</pre>
    <p><small>This note was unlocked at {{.UnlockTime}}.</small></p>
</body>
</html>
`))

	data := struct {
		Content    string
		UnlockTime string
	}{
		Content:    string(plaintext),
		UnlockTime: note.UnlockAt.Format(time.RFC1123),
	}

	if err := tmpl.Execute(w, data); err != nil {
		logger.Error("Failed to render template", "error", err)
	}
}

// handleIndex handles the GET / endpoint
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Serve the index.html file
	indexPath := filepath.Join(s.staticDir, "index.html")

	// Check if the file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		s.logger.Error("Index file not found", "path", indexPath)
		http.Error(w, "Index file not found", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, indexPath)
}
