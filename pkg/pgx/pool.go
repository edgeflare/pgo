package pgx

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolManager manages one or more named *pgxpool.Pool's.
type PoolManager struct {
	pools  map[string]*pgxpool.Pool
	active string
	mu     sync.RWMutex
}

// Pool represents a named connection configuration.
type Pool struct {
	Config     *pgxpool.Config // Takes precedence over ConnString
	Name       string
	ConnString string // Used if Config is nil
}

var (
	ErrPoolNotFound      = errors.New("connection pool not found")
	ErrPoolAlreadyExists = errors.New("connection pool already exists")
	// // For singleton pool manager instance
	// instance             *PoolManager
	// once                 sync.Once
)

// NewPoolManager returns a new connection manager.
func NewPoolManager() *PoolManager {
	return &PoolManager{pools: make(map[string]*pgxpool.Pool)}
}

// Add creates and adds a new connection pool. If `setActive=true` the connection is set as active/default connection
func (m *PoolManager) Add(ctx context.Context, cfg Pool, setActive ...bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.pools[cfg.Name]; ok {
		return ErrPoolAlreadyExists
	}

	pool, err := m.createPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("pgx: %w", err)
	}

	m.pools[cfg.Name] = pool

	// Check if `setActive` is provided and set to true
	if len(setActive) > 0 && setActive[0] {
		m.active = cfg.Name
	} else if m.active == "" {
		m.active = cfg.Name // Set active if none exists
	}

	return nil
}

// Get returns a connection pool by name.
func (m *PoolManager) Get(name string) (*pgxpool.Pool, error) {
	m.mu.RLock()
	pool, ok := m.pools[name]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrPoolNotFound
	}
	return pool, nil
}

// Active returns the current active connection pool.
func (m *PoolManager) Active() (*pgxpool.Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.active == "" {
		return nil, fmt.Errorf("pgx: no active connection")
	}
	return m.pools[m.active], nil
}

// SetActive changes the active connection.
func (m *PoolManager) SetActive(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.pools[name]; !ok {
		return fmt.Errorf("pgx: connection %q not found", name)
	}
	m.active = name
	return nil
}

// Remove closes and removes a connection pool.
func (m *PoolManager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, ok := m.pools[name]
	if !ok {
		return fmt.Errorf("pgx: connection %q not found", name)
	}

	pool.Close()
	delete(m.pools, name)

	if m.active == name {
		m.active = ""
		for k := range m.pools {
			m.active = k
			break
		}
	}
	return nil
}

// Close closes all connection pools.
func (m *PoolManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.pools {
		p.Close()
	}
	m.pools = nil
	m.active = ""
}

// List returns all connection names.
func (m *PoolManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.pools))
	for name := range m.pools {
		names = append(names, name)
	}
	return names
}

func (m *PoolManager) createPool(ctx context.Context, cfg Pool) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	switch {
	case cfg.Config != nil:
		pool, err = pgxpool.NewWithConfig(ctx, cfg.Config)
	case cfg.ConnString != "":
		pool, err = pgxpool.New(ctx, cfg.ConnString)
	default:
		return nil, errors.New("either Pool or ConnString must be provided")
	}

	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping connection: %w", err)
	}

	return pool, nil
}

// // PoolManagerSingleton returns the singleton pool manager instance.
// func PoolManagerSingleton() *PoolManager {
// 	once.Do(func() {
// 		instance = &PoolManager{pools: make(map[string]*pgxpool.Pool)}
// 	})
// 	return instance
// }
