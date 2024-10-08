# pgo (`/pɪɡəʊ/`): utils for Postgres and net/http.Handler

> Whenever I find myself copying a snippet from one Postgres+Go project to another, I move the reusable part to this repository. APIs are likely to change frequently, making it unsuitable for most projects. If you find any bit useful, instead of importing, maybe copy the relevant code or make your own fork.

## It can be useful if you:
- want to build lightweight servers without depending on full-fledged frameworks
- use PostgREST and especially leverage its [authorization pattern](https://docs.postgrest.org/en/latest/explanations/db_authz.html)
- use [custom OIDC token claims](https://zitadel.com/docs/apis/openidoauth/claims#custom-claims) for [Postgres Row Level Security (RLS)](https://www.postgresql.org/docs/13/ddl-rowsecurity.html). pgo passes [net/http](https://pkg.go.dev/net/http) Authorization header to underlying [pgxpool.Conn](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#Conn.Conn)

## Example usage

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edgeflare/pgo"
	mw "github.com/edgeflare/pgo/middleware"
	"github.com/edgeflare/pgo/pkg/util"
)

func main() {
	port := flag.Int("port", 8080, "port to run the server on")
	flag.Parse()

	r := pgo.NewRouter()

	// GET /health
	r.Handle("GET /health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pgo.Text(w, http.StatusOK, "OK")
	}))

	// Group with prefix "/api/v1"
	apiv1 := r.Group("/api/v1")

	// optional middleware with default options
	apiv1.Use(mw.RequestID)
	apiv1.Use(mw.LoggerWithOptions(nil))
	apiv1.Use(mw.CORSWithOptions(nil))

	// OIDC middleware for authentication
	oidcConfig := mw.OIDCProviderConfig{
		ClientID:     os.Getenv("PGO_OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("PGO_OIDC_CLIENT_SECRET"),
		Issuer:       os.Getenv("PGO_OIDC_ISSUER"),
	}
	apiv1.Use(mw.VerifyOIDCToken(oidcConfig))

	// pgxpool.Pool
	pool, err := pgo.InitDefaultPool(os.Getenv("PGO_POSTGRES_CONN_STRING"))
	if err != nil {
		log.Fatalln(err)
	}

	// Use Postgres middleware to attach a pgxpool.Conn to the request context for authorized users
	pgmw := mw.Postgres(pool, mw.PgOIDCAuthz(
		oidcConfig,
		util.GetEnvOrDefault("PGO_POSTGRES_OIDC_ROLE_CLAIM_KEY", ".policy.pgrole")),
	)
	apiv1.Use(pgmw)

	// Below `GET /api/v1/mypgrole` queries for, and responds with,
	// session_user, current_user using the pgxpool.Conn attached by the Postgres middleware
	apiv1.Handle("GET /mypgrole", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, conn, err := pgo.ConnWithRole(r)
		if err != nil {
			pgo.Error(w, http.StatusInternalServerError, err.Error())
		}
		defer conn.Release()

		var session_user, current_user string
		pgErr := conn.QueryRow(r.Context(), "SELECT session_user").Scan(&session_user)
		if pgErr != nil {
			pgo.Error(w, http.StatusInternalServerError, pgErr.Error())
		}
		pgErr = conn.QueryRow(r.Context(), "SELECT current_user").Scan(&current_user)
		if pgErr != nil {
			pgo.Error(w, http.StatusInternalServerError, pgErr.Error())
		}

		role := map[string]string{
			// role with which initial connection to database is established
			"session_user": session_user,
			// role with which application query is performed to process this particular request
			"current_user": current_user,
			"user_sub":     user.Subject,
		}

		pgo.JSON(w, http.StatusOK, role)
	}))

	// Run server in a goroutine
	go func() {
		if err := r.ListenAndServe(fmt.Sprintf(":%d", *port)); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	fmt.Printf("Server is running on port %d\n", *port)

	// Set up signal handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for SIGINT or SIGTERM
	<-stop

	fmt.Println("Shutting down server...")

	// Create a deadline for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := r.Shutdown(ctx); err != nil {
		fmt.Printf("server forced to shutdown: %s", err)
	}
	fmt.Println("Server gracefully stopped")
}
```

See more [examples](./examples/):
- [Publish Postgres CDC to MQTT](./examples/postgres-cdc-mqtt/)

## Contributing
Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## License
Apache License 2.0
