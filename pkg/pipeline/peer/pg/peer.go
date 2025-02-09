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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PeerPG implements source and sink functionality for PostgreSQL database
type PeerPG struct {
	pool        *pgxpool.Pool
	conn        *pgconn.PgConn
	schemaCache *schema.Cache
	pipeline.Peer
}

func (p *PeerPG) Connect(config json.RawMessage, args ...any) error {
	var cfg struct {
		ConnString string `json:"connString"`
	}
	var err error
	ctx := context.Background()

	if err = json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}

	connString := cfg.ConnString
	if connString == "" {
		return fmt.Errorf("missing connection string in config")
	}

	// Check if this is a replication connection
	if strings.Contains(connString, "replication=database") {
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

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("error connecting to database: %w", err)
	}

	p.pool = pool
	errChan := make(chan error, 1) // channel to receive errors from schema cache reload goroutine

	// init / sync schemaCache in the background
	go func() {
		conn, err := p.pool.Acquire(ctx)
		if err != nil {
			errChan <- fmt.Errorf("failed to acquire connection: %w", err)
			return
		}
		// defer conn.Release() // errors: conn closed

		p.schemaCache = schema.NewCache(conn)
		notifyCtx := context.Background()

		if err := p.schemaCache.Init(notifyCtx); err != nil {
			errChan <- fmt.Errorf("failed to initialize schema cache notifications: %w", err)
			return
		}

		errChan <- nil // succeeded
	}()

	// Wait for the goroutine to complete or timeout
	select {
	case err := <-errChan:
		if err != nil {
			p.pool.Close()
			return err
		}
	case <-time.After(5 * time.Second): // might adjust
		p.pool.Close()
		return fmt.Errorf("timeout waiting for schema cache initialization")
	}

	return nil
}

func (p *PeerPG) Pub(event pglogrepl.CDC, args ...any) error {
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
	switch op {
	case "c":
		if err := pgx.InsertRow(ctx, p.pool, tableName, event.Payload.After, schemaName); err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}

	case "u":
		where := map[string]any{}

		// Get table from schema cache
		table, exists := p.schemaCache.Table(fullTableName)
		if !exists {
			return fmt.Errorf("table %s not found in schema cache", fullTableName)
		}

		if event.Payload.Before != nil {
			for _, pkColumn := range table.PrimaryKey {
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

func (p *PeerPG) Sub(args ...any) (<-chan pglogrepl.CDC, error) {
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
	cleanChan := make(chan pglogrepl.CDC)
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
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorPostgres, &PeerPG{})
}
