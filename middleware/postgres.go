package middleware

import (
	"context"
	"net/http"

	"github.com/edgeflare/pgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// var (
// 	defaultPool *pgxpool.Pool
// )

// Postgres middleware attaches a connection from pool to the request context if the http request user is authorized.
func Postgres(pool *pgxpool.Pool, authorizers ...AuthzFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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
				conn, err := pool.Acquire(r.Context())
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

// // PostgresConfig holds configuration for the Postgres connection pool
// type PgConfig struct {
// 	// ConnString is the libpq connection string
// 	// see https://www.postgresql.org/docs/current/libpq-connect.html
// 	// if password contains special characters, use KEYWORD-VALUE format
// 	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING-KEYWORD-VALUE
// 	// if parsing ConnString fails, it falls back to libpq environment variables
// 	// PGPASSWORD is required
// 	ConnString string       `json:"conn_string"`
// 	PoolConfig PgPoolConfig `json:"pool_config,omitempty"`
// }

// // PgPoolConfig holds the configuration for the PostgreSQL connection pool.
// type PgPoolConfig struct {
// 	MaxConns          int32         `json:"max_conns,omitempty"`
// 	MinConns          int32         `json:"min_conns,omitempty"`
// 	MaxConnLifetime   time.Duration `json:"max_conn_lifetime,omitempty"`
// 	MaxConnIdleTime   time.Duration `json:"max_conn_idle_time,omitempty"`
// 	HealthCheckPeriod time.Duration `json:"health_check_period,omitempty"`
// }

// // InitPgPool initializes the default PostgreSQL connection pool.
// // returns an error if the connection pool cannot be initialized
// func InitPgPool(config *PgConfig) error {
// 	if config == nil {
// 		config = defaultPgConfig()
// 	}

// 	// parse the connection string
// 	poolConfig, err := pgxpool.ParseConfig(config.ConnString)
// 	if err != nil {
// 		log.Println("Failed to parse connection string. Trying libpq environment variables", err)
// 		// check if PGPASSWORD environment variable is set
// 		if os.Getenv("PGPASSWORD") != "" {
// 			// Construct the connection string in KEY=VALUE format
// 			keyValConnString := fmt.Sprintf(
// 				"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
// 				util.GetEnvOrDefault("PGHOST", "localhost"),
// 				util.GetEnvOrDefault("PGPORT", "5432"),
// 				util.GetEnvOrDefault("PGDATABASE", "postgres"),
// 				util.GetEnvOrDefault("PGUSER", "postgres"),
// 				os.Getenv("PGPASSWORD"),
// 				util.GetEnvOrDefault("PGSSLMODE", "require"),
// 			)

// 			poolConfig, err = pgxpool.ParseConfig(keyValConnString)
// 			if err != nil {
// 				log.Fatal("Failed to parse connection string", err)
// 			}
// 		} else {
// 			log.Fatal("PGPASSWORD environment variable is not set")
// 		}
// 	}

// 	if config.PoolConfig.MaxConns == 0 {
// 		poolConfig.MaxConns = 10
// 	}
// 	if config.PoolConfig.MinConns == 0 {
// 		poolConfig.MinConns = 1
// 	}
// 	if config.PoolConfig.MaxConnLifetime == 0 {
// 		poolConfig.MaxConnLifetime = 30 * time.Minute
// 	}
// 	if config.PoolConfig.MaxConnIdleTime == 0 {
// 		poolConfig.MaxConnIdleTime = 5 * time.Minute
// 	}
// 	if config.PoolConfig.HealthCheckPeriod == 0 {
// 		poolConfig.HealthCheckPeriod = 1 * time.Minute
// 	}

// 	defaultPool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
// 	if err != nil {
// 		log.Fatal("Unable to connect to database", err)
// 	}

// 	// background goroutine to periodically check the health of the connections.
// 	go func() {
// 		for {
// 			time.Sleep(poolConfig.HealthCheckPeriod)
// 			err := defaultPool.Ping(context.Background())
// 			if err != nil {
// 				log.Printf("Connection pool health check failed: %v", err)
// 			}
// 		}
// 	}()

// 	return nil
// }

// // DefaultPgPool returns the default PostgreSQL connection pool.
// func DefaultPool() *pgxpool.Pool {
// 	return defaultPool
// }

// // defaultPgConfig returns the default PostgreSQL connection pool configuration.
// func defaultPgConfig() *PgConfig {
// 	return &PgConfig{
// 		ConnString: os.Getenv("PGO_POSTGRES_CONN_STRING"),
// 		PoolConfig: PgPoolConfig{},
// 	}
// }
