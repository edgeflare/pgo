package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pgx"
	"github.com/edgeflare/pgo/pkg/pgx/schema"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PeerPG is Postgres Peer
type PeerPG struct {
	pipeline.Peer
	pool        *pgxpool.Pool
	schemaCache map[string]schema.Table
	mu          sync.RWMutex
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

	tableName := event.Payload.Source.Table
	schemaName := event.Payload.Source.Schema

	if tableName == "" {
		return fmt.Errorf("table name not found in CDC event")
	}

	ctx := context.Background()

	op := event.Payload.Op
	switch op {
	case "c":
		if err := pgx.InsertRow(ctx, p.pool, tableName, event.Payload.After, schemaName); err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}
	case "u":
		// derive where clause
		where := map[string]any{}
		// get table schema from cache
		p.mu.RLock()
		table, exists := p.schemaCache[tableName]
		p.mu.RUnlock()

		// if table isn't in cache, try schema.Load.
		if !exists {
			conn, err := p.pool.Acquire(ctx)
			if err != nil {
				return fmt.Errorf("failed to acquire database connection: %w", err)
			}
			defer conn.Release()

			schemaMap, err := schema.Load(ctx, conn.Conn(), schemaName)
			if err != nil {
				return fmt.Errorf("failed to load schema for table %s: %w", tableName, err)
			}

			table, exists = schemaMap[tableName]
			if !exists {
				return fmt.Errorf("table %s not found in loaded schema", tableName)
			}

			// Store the loaded schema in the cache
			p.mu.Lock()
			for name, tbl := range schemaMap {
				p.schemaCache[name] = tbl
			}
			p.mu.Unlock()
		}

		if event.Payload.Before != nil {
			// Use primary keys from schema cache
			for _, pkColumn := range table.PrimaryKey {
				if val, ok := event.Payload.Before.(map[string]any)[pkColumn]; ok {
					where[pkColumn] = val
				}
			}
			if len(where) == 0 {
				return fmt.Errorf("no primary key values found in Before payload")
			}
		} else {
			return fmt.Errorf("Before data missing for update operation")
		}

		if err := pgx.UpdateRow(ctx, p.pool, tableName, event.Payload.After, where, schemaName); err != nil {
			return fmt.Errorf("failed to update row: %w", err)
		}
	case "d":
		fmt.Println("TODO: implement")
	default:
		return fmt.Errorf("unknown operation")
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
