package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pgx"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pool is the connection pool for the PeerPG instance
type PeerPG struct {
	pipeline.Peer
	pool *pgxpool.Pool
}

func (p *PeerPG) Connect(config json.RawMessage, args ...any) error {
	var cfg struct {
		ConnString string `json:"connString"`
	}

	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}

	connString := cfg.ConnString
	if connString == "" {
		return fmt.Errorf("missing connection string in config")
	}

	// Check if this is a replication connection
	if strings.Contains(connString, "replication=database") {
		// Skip pool creation for replication connections
		return nil
	}

	// For non-replication connections, create a connection pool
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return fmt.Errorf("error parsing connString: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return fmt.Errorf("error connecting to database: %w", err)
	}

	p.pool = pool
	return nil
}

func (p *PeerPG) Pub(event pglogrepl.CDC, args ...any) error {
	if p.pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Skip if there's no After data (e.g., for DELETE operations)
	if event.Payload.After == nil {
		return nil
	}

	if event.Payload.Source.Table == "" {
		return fmt.Errorf("table name not found in CDC event")
	}

	// Convert interface{} to map[string]any
	afterData, ok := event.Payload.After.(map[string]any)
	if !ok {
		return fmt.Errorf("After data is not in expected format map[string]any")
	}

	ctx := context.Background()
	if err := pgx.InsertRow(ctx, p.pool, event.Payload.Source.Table, afterData, event.Payload.Source.Schema); err != nil {
		return fmt.Errorf("failed to insert row: %w", err)
	}

	return nil
}

func (p *PeerPG) Sub(args ...any) (<-chan pglogrepl.CDC, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("connection string required as first argument")
	}

	// Extract connection string and publication tables from args
	connString, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("first argument must be a connection string")
	}

	// Get publication tables from remaining args
	var publicationTables []string
	for _, arg := range args[1:] {
		if tableName, ok := arg.(string); ok {
			publicationTables = append(publicationTables, tableName)
		}
	}

	if len(publicationTables) == 0 {
		return nil, fmt.Errorf("at least one publication table must be specified")
	}

	// Establish replication connection
	conn, err := pgconn.Connect(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Start CDC streaming
	cdcChan, err := pglogrepl.Main(context.Background(), conn, publicationTables...)
	if err != nil {
		conn.Close(context.Background())
		return nil, fmt.Errorf("failed to start CDC streaming: %w", err)
	}

	// Create a new channel to handle connection cleanup
	cleanChan := make(chan pglogrepl.CDC)
	go func() {
		defer close(cleanChan)
		defer conn.Close(context.Background())

		for event := range cdcChan {
			select {
			case cleanChan <- event:
			case <-context.Background().Done():
				return
			}
		}
	}()

	return cleanChan, nil
}

func (p *PeerPG) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePubSub
}

func (p *PeerPG) Disconnect() error {
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorPostgres, &PeerPG{})
}
