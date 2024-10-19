package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/edgeflare/pgo/pkg/rag"
	"github.com/jackc/pgx/v5"
)

func main() {
	// insertEmbeddings()
	generateWithRetrieval()
}

func generateWithRetrieval() {
	// Database connection parameters
	connConfig, err := pgx.ParseConfig("postgres://postgres:secret@localhost:5431/testdb")
	if err != nil {
		log.Fatalf("unable to parse connection config: %v", err)
	}

	ctx := context.Background()

	// Create a new connection to the database
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	// Create a new RAG client with default configuration
	client := rag.NewClient(conn, rag.DefaultConfig())

	response, err := client.Generate(ctx, "what is the cat doing?")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("%s", response)
}

func insertEmbeddings() {
	// Database connection parameters
	connConfig, err := pgx.ParseConfig("postgres://postgres:secret@localhost:5431/testdb")
	if err != nil {
		log.Fatalf("unable to parse connection config: %v", err)
	}

	ctx := context.Background()

	// Create a new connection to the database
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	// Create a new RAG client with default configuration
	client := rag.NewClient(conn, rag.DefaultConfig())

	// Sample input texts
	input := []string{
		"The dog is barking",
		"The cat is purring",
		"The bear is growling",
		// Add more examples here
	}

	// Fetch embeddings from the API
	embeddingsResponse, err := client.FetchEmbeddings(ctx, input)
	if err != nil {
		log.Fatalf("failed to fetch embeddings: %v", err)
	}

	// Create metadata for all embeddings
	metadata := json.RawMessage(`{
		"source": "sample",
		"batch": "animal_sounds",
		"timestamp": "2024-10-19"
	}`)

	// Create tags for all embeddings
	tags := []string{"animal", "sound", "example"}

	// Create vector embeddings using the helper function
	embeddings, err := rag.ToVectorEmbedding(input, embeddingsResponse, tags, metadata)
	if err != nil {
		log.Fatalf("failed to create embeddings: %v", err)
	}

	// Insert embeddings with progress tracking
	err = client.InsertEmbeddings(ctx, embeddings, 1)
	if err != nil {
		log.Fatalf("failed to insert embeddings: %v", err)
	}

	fmt.Println("\nAll embeddings inserted successfully!")
}
