package pg_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/edgeflare/pgo/pkg/pg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testConnStr    = "postgres://postgres:secret@localhost:5432/testdb"
	invalidConnStr = "postgres://invalid:invalid@localhost:5432/invalid_db"
)

// getTestConfig returns a test configuration
func getTestConfig(name string) pg.PoolConfig {
	return pg.PoolConfig{
		Name:        name,
		ConnString:  testConnStr,
		MaxConns:    5,
		MinConns:    1,
		MaxIdleTime: time.Minute * 5,
		MaxLifetime: time.Minute * 30,
		HealthCheck: time.Minute * 1,
	}
}

// PoolSuite holds resources for pool manager tests
type PoolSuite struct {
	ctx context.Context
	pm  *pg.PoolManager
	t   testing.TB
}

// NewPoolSuite creates a new pool test suite instance
func NewPoolSuite(t testing.TB) *PoolSuite {
	return &PoolSuite{
		ctx: context.Background(),
		pm:  pg.GetPoolManager(),
		t:   t,
	}
}

func TestPoolManager(t *testing.T) {
	suite := NewPoolSuite(t)
	// defer suite.Cleanup()

	t.Run("singleton pattern", func(t *testing.T) {
		manager1 := pg.GetPoolManager()
		manager2 := pg.GetPoolManager()

		assert.Same(t, manager1, manager2, "should return same instance")
	})

	t.Run("pool lifecycle", func(t *testing.T) {
		// Test pool creation
		cfg := getTestConfig("lifecycle_test")
		require.NoError(t, suite.pm.Add(suite.ctx, cfg))

		// Test pool retrieval
		p, err := suite.pm.Get("lifecycle_test")
		require.NoError(t, err)
		require.NotNil(t, p)

		// Verify pool is functional
		var result int
		require.NoError(t, p.QueryRow(suite.ctx, "SELECT 1").Scan(&result))
		assert.Equal(t, 1, result)

		// Test pool removal
		require.NoError(t, suite.pm.Remove("lifecycle_test"))
		_, err = suite.pm.Get("lifecycle_test")
		assert.Error(t, err)
	})

	t.Run("configuration validation", func(t *testing.T) {
		tests := []struct {
			name    string
			cfg     pg.PoolConfig
			wantErr bool
		}{
			{
				name:    "valid config",
				cfg:     getTestConfig("valid_config"),
				wantErr: false,
			},
			{
				name: "invalid connection string",
				cfg: pg.PoolConfig{
					Name:       "invalid_conn",
					ConnString: invalidConnStr,
				},
				wantErr: true,
			},
			{
				name: "zero connections",
				cfg: pg.PoolConfig{
					Name:       "zero_conns",
					ConnString: testConnStr,
					MaxConns:   0,
				},
				wantErr: false, // Should use default
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := suite.pm.Add(suite.ctx, tt.cfg)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("concurrent operations", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				name := fmt.Sprintf("concurrent_%d", id)
				cfg := getTestConfig(name)

				// Test concurrent add
				if err := suite.pm.Add(suite.ctx, cfg); err != nil {
					errChan <- fmt.Errorf("failed to add pool %s: %w", name, err)
					return
				}

				// Test concurrent get
				if _, err := suite.pm.Get(name); err != nil {
					errChan <- fmt.Errorf("failed to get pool %s: %w", name, err)
					return
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			t.Errorf("concurrent operation error: %v", err)
		}

		// Verify final state
		pools := suite.pm.List()
		assert.Equal(t, numGoroutines, len(pools))
	})

	t.Run("pool listing", func(t *testing.T) {
		// Add multiple pools
		poolNames := []string{"list_test1", "list_test2", "list_test3"}
		for _, name := range poolNames {
			require.NoError(t, suite.pm.Add(suite.ctx, getTestConfig(name)))
		}

		// Verify listing
		list := suite.pm.List()
		assert.Equal(t, len(poolNames), len(list))
		for _, name := range poolNames {
			assert.Contains(t, list, name)
		}
	})
}
