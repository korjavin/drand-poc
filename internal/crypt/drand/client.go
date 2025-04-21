package drand

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/drand/drand/client"
	"github.com/drand/drand/client/http"
)

// DefaultChainHash is the hash of the drand chain info
const DefaultChainHash = "8990e7a9aaed2ffed73dbd7092123d6f289930540d7651336225dc172e51b2ce"

// Client is a wrapper around drand client
type Client struct {
	client client.Client
}

// NewClient creates a new drand client
func NewClient() (*Client, error) {
	// Use the public HTTP endpoints
	urls := []string{
		"https://api.drand.sh",
		"https://drand.cloudflare.com",
	}

	// Decode the chain hash from hex
	chainHash, err := hex.DecodeString(DefaultChainHash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode chain hash: %w", err)
	}

	// Create a new drand client with HTTP clients
	c, err := client.New(
		client.From(http.ForURLs(urls, chainHash)...),
		client.WithChainHash(chainHash),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create drand client: %w", err)
	}

	return &Client{client: c}, nil
}

// FetchRandomness fetches randomness for a specific round
func (c *Client) FetchRandomness(round uint64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get the randomness for the specified round
	result, err := c.client.Get(ctx, round)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch randomness: %w", err)
	}

	return result.Randomness(), nil
}
