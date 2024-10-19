package rag

import (
	"github.com/edgeflare/pgo/pkg/util"
	"github.com/jackc/pgx/v5"
)

// Config holds the configuration for the RAG package
type Config struct {
	TableName  string
	Dimensions int
	ModelId    string
	ApiUrl     string
	ApiKey     string
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		ModelId:    "llama3.2:3b",
		ApiKey:     util.GetEnvOrDefault("RAG_API_KEY", ""),
		TableName:  "embeddings",
		Dimensions: 3072, //  1536 for OpenAI
		ApiUrl:     util.GetEnvOrDefault("RAG_API_URL", "http://127.0.0.1:11434"),
	}
}

// Client handles the RAG operations
type Client struct {
	conn   *pgx.Conn
	config Config
}

// NewClient creates a new RAG client
func NewClient(conn *pgx.Conn, config Config) *Client {
	return &Client{
		conn:   conn,
		config: config,
	}
}
