package rag

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

// VectorEmbedding represents a single embedding with metadata
type VectorEmbedding struct {
	ID        int64            `json:"id"`
	Tags      *[]string        `json:"tags,omitempty"`
	Metadata  *json.RawMessage `json:"metadata,omitempty"`
	Content   string           `json:"content"`
	Embedding pgvector.Vector  `json:"embedding"`
}

// CreateTable creates the necessary database extensions and table
func (c *Client) CreateTable(ctx context.Context) error {
	_, err := c.conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	err = pgxvector.RegisterTypes(ctx, c.conn)
	if err != nil {
		return fmt.Errorf("failed to register vector types: %w", err)
	}

	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGINT PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
		tags TEXT[],
		metadata JSONB,
		content TEXT,
		embedding VECTOR(%v)
	)`, c.config.TableName, c.config.Dimensions)

	_, err = c.conn.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// InsertEmbeddings takes a slice of VectorEmbedding and inserts them into the database
// using the provided configuration
func (c *Client) InsertEmbeddings(ctx context.Context, embeddings []VectorEmbedding, batchSize ...int) error {
	if len(embeddings) == 0 {
		return nil
	}

	// Ensure table exists
	if err := c.CreateTable(ctx); err != nil {
		return fmt.Errorf("failed to set up database: %w", err)
	}

	// Prepare the query
	query := fmt.Sprintf(`
		INSERT INTO %s (content, embedding, tags, metadata)
		VALUES ($1, $2, $3, $4)`,
		c.config.TableName,
	)

	// Process embeddings in batches
	for i := 0; i < len(embeddings); i += batchSize[0] {
		end := i + batchSize[0]
		if end > len(embeddings) {
			end = len(embeddings)
		}

		batch := embeddings[i:end]

		// Begin transaction for this batch
		tx, err := c.conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Create a new batch
		pgxBatch := &pgx.Batch{}
		for _, embedding := range batch {
			pgxBatch.Queue(query,
				embedding.Content,
				embedding.Embedding,
				embedding.Tags,
				embedding.Metadata,
			)
		}

		// Execute batch
		batchResults := tx.SendBatch(ctx, pgxBatch)

		// Process all results before closing
		for j := 0; j < pgxBatch.Len(); j++ {
			_, err := batchResults.Exec()
			if err != nil {
				batchResults.Close()
				tx.Rollback(ctx)
				return fmt.Errorf("failed to insert embedding at index %d: %w", i+j, err)
			}
		}

		// Close batch results before committing
		if err := batchResults.Close(); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to close batch results: %w", err)
		}

		// Commit transaction
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return nil
}

// Retrieve returns []VectorEmbedding similar to a query using the RAG API and database
func (c *Client) Retrieve(ctx context.Context, query string, limit int) ([]VectorEmbedding, error) {
	// Get embeddings for the query
	embeddingResponse, err := c.FetchEmbeddings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch embeddings: %w", err)
	}

	if len(embeddingResponse.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned from API")
	}

	// Construct the query using the configured table name
	queryStr := fmt.Sprintf(
		"SELECT id, content, tags, metadata, embedding FROM %s ORDER BY embedding <=> $1 LIMIT $2",
		c.config.TableName,
	)

	// Execute the query
	rows, err := c.conn.Query(ctx, queryStr,
		pgvector.NewVector(embeddingResponse.Data[0].Embedding),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to execute similarity search: %w", err)
	}
	defer rows.Close()

	// Process the results
	var results []VectorEmbedding
	for rows.Next() {
		var doc VectorEmbedding
		if err := rows.Scan(&doc.ID, &doc.Content, &doc.Tags, &doc.Metadata, &doc.Embedding); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return results, nil
}

// Helper function to create embeddings from API response
func ToVectorEmbedding(texts []string, response EmbeddingsResponse, tags []string, metadata json.RawMessage) ([]VectorEmbedding, error) {
	if len(texts) != len(response.Data) {
		return nil, fmt.Errorf("mismatch between texts (%d) and embeddings (%d)", len(texts), len(response.Data))
	}

	embeddings := make([]VectorEmbedding, len(texts))
	for i, text := range texts {
		vector := pgvector.NewVector(response.Data[i].Embedding)
		embeddings[i] = VectorEmbedding{
			Content:   text,
			Embedding: vector,
			Tags:      &tags,
			Metadata:  &metadata,
		}
	}

	return embeddings, nil
}