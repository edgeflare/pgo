package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Manager manages multiple named *pgxpool.Pool's.
type Manager struct {
	mu     sync.RWMutex
	pools  map[string]*pgxpool.Pool
	active string
}

// Config represents a named connection configuration.
type Config struct {
	Name       string
	ConnString string          // Used if PoolConfig is nil
	PoolConfig *pgxpool.Config // Takes precedence over ConnString
}

var (
	ErrPoolNotFound      = errors.New("connection pool not found")
	ErrPoolAlreadyExists = errors.New("connection pool already exists")
	// instance             *Manager
	// once                 sync.Once
)

// NewManager returns a new connection manager.
func NewManager() *Manager {
	return &Manager{pools: make(map[string]*pgxpool.Pool)}
}

// Add creates and adds a new connection pool. If `setActive=true` the connection is set as active/default connection
func (m *Manager) Add(ctx context.Context, cfg Config, setActive ...bool) error {
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
func (m *Manager) Get(name string) (*pgxpool.Pool, error) {
	m.mu.RLock()
	pool, ok := m.pools[name]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrPoolNotFound
	}
	return pool, nil
}

// Active returns the current active connection pool.
func (m *Manager) Active() (*pgxpool.Pool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.active == "" {
		return nil, fmt.Errorf("pgx: no active connection")
	}
	return m.pools[m.active], nil
}

// SetActive changes the active connection.
func (m *Manager) SetActive(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.pools[name]; !ok {
		return fmt.Errorf("pgx: connection %q not found", name)
	}
	m.active = name
	return nil
}

// Remove closes and removes a connection pool.
func (m *Manager) Remove(name string) error {
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
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.pools {
		p.Close()
	}
	m.pools = nil
	m.active = ""
}

// List returns all connection names.
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.pools))
	for name := range m.pools {
		names = append(names, name)
	}
	return names
}

func (m *Manager) createPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	if cfg.PoolConfig != nil {
		pool, err = pgxpool.NewWithConfig(ctx, cfg.PoolConfig)
	} else if cfg.ConnString != "" {
		pool, err = pgxpool.New(ctx, cfg.ConnString)
	} else {
		return nil, errors.New("either PoolConfig or ConnString must be provided")
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

// // ManagerSingleton returns the singleton pool manager instance.
// func ManagerSingleton() *Manager {
// 	once.Do(func() {
// 		instance = &Manager{pools: make(map[string]*pgxpool.Pool)}
// 	})
// 	return instance
// }
