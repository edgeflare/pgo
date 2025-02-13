package pg

import (
	"context"
	"encoding/json"
	"errors"
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

var (
	errNotConnected      = errors.New("not connected")
	errMissingConnString = errors.New("missing connection string")
)

// PeerPG implements the source and sink functionality for PostgreSQL database
type PeerPG struct {
	pool        *pgxpool.Pool
	conn        *pgconn.PgConn
	cfg         Config
	schemaCache *schema.Cache
	tables      map[string]schema.Table
}

type Config struct {
	ConnString string `json:"connString"`
	pglogrepl.Config
}

func (p *PeerPG) Connect(config json.RawMessage, _ ...any) error {
	if err := json.Unmarshal(config, &p.cfg); err != nil {
		return fmt.Errorf("config parse: %w", err)
	}
	if p.cfg.ConnString == "" {
		return errMissingConnString
	}

	ctx := context.Background()
	// check if this is a replication connection
	if strings.Contains(p.cfg.ConnString, "replication=database") {
		var err error
		p.conn, err = pgconn.Connect(ctx, p.cfg.ConnString)
		if err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL server %w", err)
		}
		return nil
	}

	// for non-replication connections, create a connection pool
	pool, err := pgxpool.New(ctx, p.cfg.ConnString)
	if err != nil {
		return err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("error connecting to database: %w", err)
	}
	p.pool = pool

	// initialize schema cache
	// TODO
	// caching the schema potentiall could improve Pub perf
	schemaCache, err := schema.NewCache(p.cfg.ConnString)
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

func (p *PeerPG) Sub(_ ...any) (<-chan cdc.Event, error) {
	if p.conn == nil {
		return nil, errNotConnected
	}

	replConfig := &pglogrepl.Config{
		Publication:       p.cfg.Publication,
		ReplicationSlot:   p.cfg.ReplicationSlot,
		Plugin:            p.cfg.Plugin,
		StandbyPeriod:     p.cfg.StandbyPeriod,
		PublicationTables: p.cfg.PublicationTables,
	}

	return pglogrepl.Stream(context.Background(), p.conn, replConfig)
}

func (p *PeerPG) Pub(event cdc.Event, _ ...any) error {
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
	case cdc.OpCreate:
		if err := pgx.InsertRow(ctx, p.pool, tableName, event.Payload.After, schemaName); err != nil {
			return fmt.Errorf("failed to insert row: %w", err)
		}
	case cdc.OpUpdate:
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
	case cdc.OpDelete:
		fmt.Println("TODO: implement")
	case cdc.OpTruncate:
	default:
		return fmt.Errorf("unknown operation")
	}

	return nil
}

func (p *PeerPG) Type() pipeline.ConnectorType {
	return pipeline.ConnectorTypePubSub
}

func (p *PeerPG) Disconnect() error {
	ctx := context.Background()
	if p.pool != nil {
		p.pool.Close()
	}
	if p.conn != nil {
		return p.conn.Close(ctx)
	}
	return nil
}

func init() {
	pipeline.RegisterConnector(pipeline.ConnectorPostgres, &PeerPG{})
}
