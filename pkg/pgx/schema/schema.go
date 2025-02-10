package schema

import (
	"context"
	"fmt"
	"sync"

	pg "github.com/edgeflare/pgo/pkg/pgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// Following PostgREST's notification convention
	// https://docs.postgrest.org/en/stable/references/schema_cache.html
	reloadChannel = "pgo" // pgrst for PostgREST
	reloadPayload = "reload schema"
)

type Table struct {
	Schema      string
	Name        string
	Columns     []Column
	PrimaryKeys []string
	ForeignKeys []ForeignKey
}

type Column struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
}

type ForeignKey struct {
	Column           string
	ReferencedTable  string
	ReferencedColumn string
}

type Cache struct {
	pool   *pgxpool.Pool    // for querying database
	conn   *pgx.Conn        // dedicated connection for notifications
	tables map[string]Table // map key: schema_name.table_name
	watch  chan map[string]Table
	cancel context.CancelFunc
	mu     sync.RWMutex
}

func NewCache(connString string) (*Cache, error) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	notificationConn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("pool.Acquire: %w", err)
	}

	return &Cache{
		pool:   pool,
		conn:   notificationConn.Hijack(),
		tables: make(map[string]Table),
		watch:  make(chan map[string]Table, 1),
	}, nil
}

func (c *Cache) Init(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	if err := c.reload(ctx); err != nil {
		cancel()
		return fmt.Errorf("initial load: %w", err)
	}

	_, err := c.conn.Exec(ctx, "LISTEN "+reloadChannel)
	if err != nil {
		cancel()
		return fmt.Errorf("listen: %w", err)
	}

	go c.handleUpdates(ctx)
	return nil
}

func (c *Cache) Close() {
	if c.cancel != nil {
		c.cancel() // Cancel context before closing connections
	}
	if c.conn != nil {
		c.conn.Close(context.Background())
	}
	if c.pool != nil {
		c.pool.Close()
	}
	close(c.watch)
}

func (c *Cache) Watch() <-chan map[string]Table {
	return c.watch
}

func (c *Cache) handleUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			notification, err := c.conn.WaitForNotification(ctx)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					c.watch <- c.snapshot()
					fmt.Printf("notification error: %v\n", err)
					continue
				}
			}

			if notification.Payload == reloadPayload {
				if err := c.reload(ctx); err != nil {
					fmt.Printf("reload error: %v\n", err)
				}
			}
		}
	}
}

func (t *Table) fullName() string {
	return fmt.Sprintf("%s.%s", t.Schema, t.Name)
}

func (c *Cache) reload(ctx context.Context) error {
	tables, err := loadAll(ctx, c.pool)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.tables = tables
	c.mu.Unlock()

	c.watch <- c.snapshot()
	return nil
}

func loadAll(ctx context.Context, conn pg.Conn) (map[string]Table, error) {
	schemas, err := querySchemas(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("query schemas: %w", err)
	}

	tables := make(map[string]Table)
	for _, schema := range schemas {
		if isSystem(schema) {
			continue
		}

		schemaTables, err := loadSchema(ctx, conn, schema)
		if err != nil {
			return nil, fmt.Errorf("load schema %s: %w", schema, err)
		}

		for k, v := range schemaTables {
			tables[k] = v
		}
	}
	return tables, nil
}

func (c *Cache) snapshot() map[string]Table {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snap := make(map[string]Table, len(c.tables))
	for k, v := range c.tables {
		snap[k] = v
	}
	return snap
}

func loadSchema(ctx context.Context, conn pg.Conn, schema string) (map[string]Table, error) {
	rows, err := conn.Query(ctx, `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_type = 'BASE TABLE'`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string]Table)
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Schema, &t.Name); err != nil {
			return nil, err
		}

		cols, pkeys, err := queryColumns(ctx, conn, t.Schema, t.Name)
		if err != nil {
			return nil, fmt.Errorf("query columns %s.%s: %w", t.Schema, t.Name, err)
		}
		t.Columns = cols
		t.PrimaryKeys = pkeys

		fkeys, err := queryForeignKeys(ctx, conn, t.Schema, t.Name)
		if err != nil {
			return nil, fmt.Errorf("query foreign keys %s.%s: %w", t.Schema, t.Name, err)
		}
		t.ForeignKeys = fkeys

		tables[t.fullName()] = t
	}
	return tables, rows.Err()
}

func queryColumns(ctx context.Context, conn pg.Conn, schema, table string) ([]Column, []string, error) {
	rows, err := conn.Query(ctx, `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable = 'YES',
			EXISTS (
				SELECT 1 FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
					AND tc.table_schema = kcu.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
					AND tc.table_schema = $1
					AND tc.table_name = $2
					AND kcu.column_name = c.column_name
			) AS is_primary_key
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2`, schema, table)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var cols []Column
	var pkeys []string
	for rows.Next() {
		var col Column
		if err := rows.Scan(&col.Name, &col.DataType, &col.IsNullable, &col.IsPrimaryKey); err != nil {
			return nil, nil, err
		}
		cols = append(cols, col)
		if col.IsPrimaryKey {
			pkeys = append(pkeys, col.Name)
		}
	}
	return cols, pkeys, rows.Err()
}

func queryForeignKeys(ctx context.Context, conn pg.Conn, schema, table string) ([]ForeignKey, error) {
	rows, err := conn.Query(ctx, `
		SELECT
			kcu.column_name,
			ccu.table_name,
			ccu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1
			AND tc.table_name = $2`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fkeys []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(&fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn); err != nil {
			return nil, err
		}
		fkeys = append(fkeys, fk)
	}
	return fkeys, rows.Err()
}

func querySchemas(ctx context.Context, conn pg.Conn) ([]string, error) {
	rows, err := conn.Query(ctx, `SELECT schema_name FROM information_schema.schemata ORDER BY schema_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}
	return schemas, rows.Err()
}

func isSystem(schema string) bool {
	switch schema {
	case "information_schema", "pg_catalog", "pg_toast", "pg_temp_1", "pg_toast_temp_1":
		return true
	default:
		return false
	}
}
