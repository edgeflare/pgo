package rag

import (
	"context"
	"testing"
)

func TestFetchEmbedding(t *testing.T) {
	// Create a test client with default config
	config := DefaultConfig()

	ctx := context.Background()
	conn, err := setupTestDatabase(t)
	if err != nil {
		t.Fatalf("Failed to set up test database: %v", err)
	}
	defer conn.Close(ctx)

	c, err := NewClient(conn, config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test input
	input := []string{"Hello", "World"}

	// Call FetchEmbedding
	embeddings, err := c.FetchEmbedding(context.Background(), input)

	// Check for errors
	if err != nil {
		t.Fatalf("FetchEmbedding returned an error: %v", err)
	}

	// Check the number of embeddings
	if len(embeddings) != len(input) {
		t.Errorf("Expected %d embeddings, got %d", len(input), len(embeddings))
	}

	// Check that embeddings are not empty
	for i, embedding := range embeddings {
		if len(embedding) == 0 {
			t.Errorf("Embedding %d is empty", i)
		}
	}

	// Print the first few values of each embedding for manual inspection
	t.Logf("Embeddings (first 5 values):")
	for i, embedding := range embeddings {
		t.Logf("  Input '%s': %v", input[i], embedding[:5])
	}
}
