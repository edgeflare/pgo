package main

// See [docs/rag.md](../../docs/rag.md) for more information.

import (
	"context"
	"fmt"
	"log"

	"github.com/edgeflare/pgo/pkg/rag"
	"github.com/edgeflare/pgo/pkg/util"
	"github.com/jackc/pgx/v5"
)

func main() {
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, util.GetEnvOrDefault("DATABASE_URL", "postgres://postgres:secret@localhost:5432/postgres"))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	// Create a new RAG client
	client, err := rag.NewClient(conn, rag.DefaultConfig())
	client.Config.TableName = "lms.courses"
	// TODO: fix primary key data type from primary key column in contentSelectQuery

	if err != nil {
		log.Fatalf("Failed to create RAG client: %v", err)
	}

	err = client.CreateEmbedding(ctx, "SELECT id, CONCAT('title:', title, ', summary:', summary) AS content FROM lms.courses")
	// err = client.CreateEmbedding(ctx, "") // CreateEmbedding constructs content by concatenating colname:value of other columns for each row
	// err = client.CreateEmbedding(ctx) // Assumes the table has a column named `content` that contains the content for which embedding will be created
	if err != nil {
		log.Fatalf("Failed to create embeddings: %v", err)
	}

	fmt.Println("Embeddings have been successfully created.")

	// retrieval example
	input := "example input text"
	limit := 2

	results, err := client.Retrieve(ctx, input, limit)
	if err != nil {
		log.Fatalf("Failed to retrieve content: %v", err)
	}

	// Print the retrieved results
	for _, r := range results {
		fmt.Printf("ID: %v\nContent: %s\nEmbedding: %v\n", r.PK, r.Content, r.Embedding.Slice()[0])
	}
}
