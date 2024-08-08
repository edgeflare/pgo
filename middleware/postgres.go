package middleware

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/edgeflare/pgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	defaultPool *pgxpool.Pool
)

// Postgres middleware attaches a connection from pool to the request context if the http request user is authorized.
func Postgres(config PgConfig, authorizers ...AuthzFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// lazy initialization of the default pool
		if defaultPool == nil {
			InitPgPool(&config)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			for _, authorize := range authorizers {
				authzResponse, err := authorize(ctx)
				if err != nil {
					http.Error(w, "Authorization error", http.StatusInternalServerError)
					return
				}
				if authzResponse.Allowed {
					ctx = context.WithValue(ctx, pgo.PgRoleCtxKey, authzResponse.Role)
					break
				}
			}

			if pgRole, ok := ctx.Value(pgo.PgRoleCtxKey).(string); ok {
				// Acquire a connection from the default pool
				conn, err := defaultPool.Acquire(r.Context())
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				// caller should
				// defer conn.Release()

				// set the connection in the context
				ctx = context.WithValue(ctx, pgo.PgConnCtxKey, conn)
				ctx = context.WithValue(ctx, pgo.PgRoleCtxKey, pgRole)
				r = r.WithContext(ctx)
				next.ServeHTTP(w, r)
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
		})
	}
}

// PostgresConfig holds configuration for the Postgres connection pool
type PgConfig struct {
	ConnString string       `json:"conn_string"`
	PoolConfig PgPoolConfig `json:"pool_config,omitempty"`
}

// PgPoolConfig holds the configuration for the PostgreSQL connection pool.
type PgPoolConfig struct {
	MaxConns          int32         `json:"max_conns,omitempty"`
	MinConns          int32         `json:"min_conns,omitempty"`
	MaxConnLifetime   time.Duration `json:"max_conn_lifetime,omitempty"`
	MaxConnIdleTime   time.Duration `json:"max_conn_idle_time,omitempty"`
	HealthCheckPeriod time.Duration `json:"health_check_period,omitempty"`
}

// InitPgPool initializes the default PostgreSQL connection pool.
func InitPgPool(config *PgConfig) {
	if config == nil {
		config = defaultPgConfig()
	}

	poolConfig, err := pgxpool.ParseConfig(config.ConnString)
	if err != nil {
		log.Fatal("Failed to parse connection string", err)
	}

	if config.PoolConfig.MaxConns == 0 {
		poolConfig.MaxConns = 10
	}
	if config.PoolConfig.MinConns == 0 {
		poolConfig.MinConns = 1
	}
	if config.PoolConfig.MaxConnLifetime == 0 {
		poolConfig.MaxConnLifetime = 30 * time.Minute
	}
	if config.PoolConfig.MaxConnIdleTime == 0 {
		poolConfig.MaxConnIdleTime = 5 * time.Minute
	}
	if config.PoolConfig.HealthCheckPeriod == 0 {
		poolConfig.HealthCheckPeriod = 1 * time.Minute
	}

	defaultPool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal("Unable to connect to database", err)
	}

	// background goroutine to periodically check the health of the connections.
	go func() {
		for {
			time.Sleep(poolConfig.HealthCheckPeriod)
			err := defaultPool.Ping(context.Background())
			if err != nil {
				log.Printf("Connection pool health check failed: %v", err)
			}
		}
	}()
}

// DefaultPgPool returns the default PostgreSQL connection pool.
func DefaultPool() *pgxpool.Pool {
	return defaultPool
}

func defaultPgConfig() *PgConfig {
	return &PgConfig{
		ConnString: os.Getenv("PGO_POSTGRES_CONN_STRING"),
		PoolConfig: PgPoolConfig{},
	}
}
