package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
	"github.com/edgeflare/pgo/pkg/pgx"
	"github.com/edgeflare/pgo/pkg/pgx/schema"
	"github.com/edgeflare/pgo/pkg/pipeline"
	"github.com/edgeflare/pgo/pkg/pipeline/cdc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PeerPG struct {
	pool        *pgxpool.Pool  // for Pub
	conn        *pgconn.PgConn // for Sub
	schemaCache *schema.Cache
	tables      map[string]schema.Table
	pipeline.Peer
}

func (p *PeerPG) Connect(config json.RawMessage, args ...any) error {
	var cfg struct {
		ConnString string `json:"connString"`
	}
	ctx := context.Background()
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}

	connString := cfg.ConnString
	if connString == "" {
		return fmt.Errorf("missing connection string in config")
	}

	// Check if this is a replication connection
	if strings.Contains(connString, "replication=database") {
		var err error
		p.conn, err = pgconn.Connect(ctx, connString)
		if err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL server %w", err)
		}
		return nil
	}

	// For non-replication connections, create a connection pool
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return fmt.Errorf("error parsing connString: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("error connecting to database: %w", err)
	}
	p.pool = pool

	// Initialize schema cache
	schemaCache, err := schema.NewCache(connString)
	if err != nil {
		p.pool.Close()
		return fmt.Errorf("failed to create schema cache: %w", err)
	}
	p.schemaCache = schemaCache

	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := p.schemaCache.Init(initCtx); err != nil {
		p.pool.Close()
		p.schemaCache.Close()
		return fmt.Errorf("failed to initialize schema cache: %w", err)
	}

	go func() {
		for tables := range p.schemaCache.Watch() {
			p.tables = tables
		}
	}()

	return nil
}

func (p *PeerPG) Pub(event cdc.CDC, args ...any) error {
	if p.pool == nil {
		return fmt.Errorf("database connection not initialized")
	}
	if event.Payload.After == nil {
		return nil
	}

	tableName := event.Payload.Source.Table
	schemaName := event.Payload.Source.Schema
	fullTableName := fmt.Sprintf("%s.%s", schemaName, tableName)

	if tableName == "" {
		return fmt.Errorf("table name not found in CDC event")
	}

	ctx := context.Background()
	op := event.Payload.Op

	// Get table from current schema state
	table, exists := p.tables[fullTableName]
	if !exists {
		return fmt.Errorf("table %s not found in schema cache", fullTableName)
	}

	switch op {
	case "c":
		if err := pgx.InsertRow(ctx, p.pool, tableName, event.Payload.After, schemaName); err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}

	case "u":
		where := map[string]any{}
		if event.Payload.Before != nil {
			for _, pkColumn := range table.PrimaryKeys {
				if val, ok := event.Payload.Before.(map[string]any)[pkColumn]; ok {
					where[pkColumn] = val
				}
			}
			if len(where) == 0 {
				return fmt.Errorf("no primary key values found in Before payload")
			}
		} else {
			return fmt.Errorf("before data missing for update operation")
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

func (p *PeerPG) Sub(args ...any) (<-chan cdc.CDC, error) {
	// Get publication tables from args
	var publicationTables []string
	for _, arg := range args {
		if tableName, ok := arg.(string); ok {
			publicationTables = append(publicationTables, tableName)
		}
	}

	if len(publicationTables) == 0 {
		return nil, fmt.Errorf("at least one publication table must be specified")
	}

	ctx := context.Background()

	// Start CDC streaming
	cdcChan, err := pglogrepl.Main(ctx, p.conn, publicationTables...)
	if err != nil {
		p.conn.Close(ctx)
		return nil, fmt.Errorf("failed to start CDC streaming: %w", err)
	}

	// Create a new channel to handle connection cleanup
	cleanChan := make(chan cdc.CDC)
	go func() {
		defer close(cleanChan)
		defer p.conn.Close(ctx)

		for event := range cdcChan {
			select {
			case cleanChan <- event:
			case <-ctx.Done():
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
	if p.schemaCache != nil {
		p.schemaCache.Close()
	}
	if p.pool != nil {
		p.pool.Close()
	}
	if p.conn != nil {
		return p.conn.Close(context.Background())
	}
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorPostgres, &PeerPG{})
}
