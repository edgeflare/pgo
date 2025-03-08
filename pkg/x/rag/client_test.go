package rag

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestNewClient(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	client, err := NewClient(conn, Config{
		TableName:  "test_table",
		Dimensions: 3072,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	defer client.conn.Close(ctx)
}
