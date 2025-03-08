package rag

import (
	"cmp"
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
	"go.uber.org/zap"
)

// Config holds the configuration for the RAG Client
type Config struct {
	TableName          string
	TablePrimaryKeyCol string
	ModelID            string
	APIURL             string
	APIKey             string
	EmbeddingsPath     string
	GeneratePath       string
	Dimensions         int
	BatchSize          int
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		ModelID:            "llama3.2:3b",
		APIKey:             cmp.Or(os.Getenv("LLM_API_KEY"), ""),
		TableName:          "embeddings",
		TablePrimaryKeyCol: "id",
		Dimensions:         3072, // for llama3.2:3b, 1536 for OpenAI
		APIURL:             cmp.Or(os.Getenv("LLM_API_URL"), "http://127.0.0.1:11434"),
		EmbeddingsPath:     "/v1/embeddings",
		GeneratePath:       "/api/generate",
		BatchSize:          100,
	}
}

// Client handles the RAG operations
type Client struct {
	conn   *pgx.Conn
	logger *zap.Logger
	Config Config
}

// NewClient creates a new RAG client
func NewClient(conn *pgx.Conn, config Config, loggers ...*zap.Logger) (*Client, error) {
	var logger *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	} else {
		var err error
		// logger, err = zap.NewDevelopment()
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("failed to create logger: %w", err)
		}
	}

	client := &Client{
		conn:   conn,
		Config: config,
		logger: logger,
	}

	if err := client.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	return client, nil
}

func (c *Client) initialize() error {
	ctx := context.Background()

	_, err := c.conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	err = pgxvector.RegisterTypes(ctx, c.conn)
	if err != nil {
		return fmt.Errorf("failed to register vector types: %w", err)
	}

	return nil
}

// ============================================ With retry ============================================
/*
func (c *Client) ExecuteWithRetry(ctx context.Context, operation func(context.Context) error) error {
	var err error
	for attempt := 0; attempt < c.config.RetryAttempts; attempt++ {
		err = operation(ctx)
		if err == nil {
			return nil
		}
		if !isRetryableError(err) {
			return err
		}
		time.Sleep(c.config.RetryBackoff * time.Duration(attempt+1))
	}
	return fmt.Errorf("operation failed after %d attempts: %w", c.config.RetryAttempts, err)
}

func (c *Client) QueryWithMetrics(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	if c.config.TracingEnabled {
		var span trace.Span
		ctx, span = otel.Tracer("rag").Start(ctx, "QueryWithMetrics")
		defer span.End()
	}

	start := time.Now()
	rows, err := c.conn.Query(ctx, query, args...)
	duration := time.Since(start)

	if c.config.MetricsEnabled {
		// Record metrics (e.g., query duration, success/failure)
		// This could integrate with Prometheus, StatsD, etc.
		c.logger.Info("query_duration", zap.Duration("duration", duration))
	}

	return rows, err
}

// isRetryableError determines if the given error is retryable
func isRetryableError(err error) bool {
	// Check for specific error types that are retryable
	if pgErr, ok := err.(*pgconn.PgError); ok {
		// PostgreSQL error codes that are typically retryable
		retryableCodes := map[string]bool{
			"40001": true, // serialization_failure
			"40P01": true, // deadlock_detected
			"53300": true, // too_many_connections
			"53400": true, // configuration_limit_exceeded
			"08006": true, // connection_failure
			"08001": true, // sqlclient_unable_to_establish_sqlconnection
			"08004": true, // sqlserver_rejected_establishment_of_sqlconnection
		}
		return retryableCodes[pgErr.Code]
	}

	// Check for network-related errors
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}

	// By default, consider the error as non-retryable
	return false
}
*/
