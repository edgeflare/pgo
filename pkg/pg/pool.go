package pg

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig defines parameters for a database connection pool.
type PoolConfig struct {
	Name        string        // Unique identifier for the pool
	ConnString  string        // PostgreSQL connection string
	MaxConns    int32         // Maximum number of connections (default: 4)
	MinConns    int32         // Minimum number of idle connections (default: 0)
	MaxIdleTime time.Duration // Maximum time a connection can be idle (default: 30m)
	MaxLifetime time.Duration // Maximum lifetime of a connection (default: 60m)
	HealthCheck time.Duration // Interval between health checks (default: 1m)
	OnNotice    func(*pgconn.PgConn, *pgconn.Notice)
}

// PoolManager maintains a registry of named pools for database connections.
type PoolManager struct {
	pools map[string]*pgxpool.Pool
	mu    sync.RWMutex
}

var (
	instance *PoolManager
	once     sync.Once
)

// GetManager returns the singleton pool manager instance.
func GetPoolManager() *PoolManager {
	once.Do(func() {
		instance = &PoolManager{pools: make(map[string]*pgxpool.Pool)}
	})
	return instance
}

// Default pool configuration values
const (
	defaultMaxConns    int32         = 4                // pgxpool default
	defaultMinConns    int32         = 0                // pgxpool default
	defaultMaxIdleTime time.Duration = 30 * time.Minute // pgxpool default
	defaultMaxLifetime time.Duration = 60 * time.Minute // pgxpool default
	defaultHealthCheck time.Duration = 1 * time.Minute  // pgxpool default
)

// Add creates and registers a new connection pool with the given configuration.
func (m *PoolManager) Add(ctx context.Context, cfg PoolConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	poolConfig, err := pgxpool.ParseConfig(cfg.ConnString)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Apply configuration with defaults
	poolConfig.MaxConns = defaultMaxConns
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = cfg.MaxConns
	}
	poolConfig.MinConns = defaultMinConns
	if cfg.MinConns >= 0 {
		poolConfig.MinConns = cfg.MinConns
	}
	poolConfig.MaxConnIdleTime = defaultMaxIdleTime
	if cfg.MaxIdleTime > 0 {
		poolConfig.MaxConnIdleTime = cfg.MaxIdleTime
	}
	if cfg.MaxLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.MaxLifetime
	}
	if cfg.HealthCheck > 0 {
		poolConfig.HealthCheckPeriod = cfg.HealthCheck
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	m.pools[cfg.Name] = pool
	return nil
}

// Get returns the connection pool for the given database name.
// It returns an error if the pool doesn't exist.
func (m *PoolManager) Get(name string) (*pgxpool.Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if pool, ok := m.pools[name]; ok {
		return pool, nil
	}
	return nil, fmt.Errorf("pool %q not found", name)
}

// Close closes all managed connection pools.
func (m *PoolManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.pools {
		p.Close()
	}
}

// Remove closes and removes the named connection pool.
// It returns an error if the pool doesn't exist.
func (m *PoolManager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool, ok := m.pools[name]; ok {
		pool.Close()
		delete(m.pools, name)
		return nil
	}
	return fmt.Errorf("pool %q not found", name)
}

// List returns the names of all managed connection pools.
func (m *PoolManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.pools))
	for name := range m.pools {
		names = append(names, name)
	}
	return names
}
