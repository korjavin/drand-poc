package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v3"
	"github.com/korjavin/drand-poc/server"
	"github.com/korjavin/drand-poc/storage"
)

func main() {
	// Parse command-line flags
	addr := flag.String("addr", ":8083", "HTTP server address")
	dataDir := flag.String("data", "./data", "Data directory for Badger DB")
	staticDir := flag.String("static", "./frontend", "Static files directory")
	baseDomain := flag.String("base-domain", "", "Base domain for URLs (default: http://localhost:PORT)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Set up logging
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	// Create the data directory if it doesn't exist
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		logger.Error("Failed to create data directory", "error", err)
		os.Exit(1)
	}

	// Set up Badger DB
	badgerOpts := badger.DefaultOptions(*dataDir)
	badgerOpts.Logger = nil // Disable Badger's internal logger
	store, err := storage.NewBadgerStore(badgerOpts)
	if err != nil {
		logger.Error("Failed to create Badger store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Set the base domain
	if *baseDomain == "" {
		// Use the environment variable if set
		*baseDomain = os.Getenv("BASE_DOMAIN")
		if *baseDomain == "" {
			// Default to localhost with the specified port
			*baseDomain = "http://localhost" + *addr
		}
	}

	// Ensure the static directory exists
	if _, err := os.Stat(*staticDir); os.IsNotExist(err) {
		logger.Error("Static directory does not exist", "dir", *staticDir)
		os.Exit(1)
	}

	// Check if index.html exists in the static directory
	indexPath := filepath.Join(*staticDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		logger.Error("index.html not found in static directory", "path", indexPath)
		os.Exit(1)
	}

	// Create and start the server
	srv := server.NewServer(store, logger, *baseDomain, *staticDir)
	logger.Info("Starting server", "addr", *addr, "base_domain", *baseDomain)
	if err := srv.Start(*addr); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}
