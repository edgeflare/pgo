# pgo (`/pɪɡəʊ/`): utils for Postgres and net/http.Handler

## It can be useful if you:
- expose Postgres via REST API using primarily PostgREST, leverage its [authorization pattern](https://docs.postgrest.org/en/latest/explanations/db_authz.html), and want custom logic for an endpoint or two, beyond just CRUD
- use [custom OIDC token claims](https://zitadel.com/docs/apis/openidoauth/claims#custom-claims) to authorize Postgres queries (possibly with) [Postgres Row Level Security (RLS)](https://www.postgresql.org/docs/13/ddl-rowsecurity.html). pgo passes [net/http](https://pkg.go.dev/net/http) request context *eg* headers to underlying [pgxpool.Conn](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#Conn.Conn)
- wanna experiment with a [router.go](./router.go) written in ~50 lines of code dependent only on standard library. It's a wrapper around [http.ServeMux](https://pkg.go.dev/net/http#ServeMux) with helpers for route groups, middleware.

## Realworld-ish examples
- [guardian](https://github.com/edgeflare/guardian): manages [WireGuard](https://www.wireguard.com/) networks and peeers
- [fabric-oidc-proxy](https://github.com/edgeflare/fabric-oidc-proxy): allows authenticating to Hyperledger Fabric blockchain using OIDC token. It requests x509 certificate for each user from Fabric CA, and signs transactions using respective user's certificate.

```go
// minimal error handling for brevity
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

	"github.com/edgeflare/guardian/wg"
	"github.com/edgeflare/pgo"
	mw "github.com/edgeflare/pgo/middleware"

	"github.com/jackc/pgx/v5"
)

func main() {
	port := flag.Int("port", 8080, "port to run the server on")
	flag.Parse()

	r := pgo.NewRouter()

	// (optional) middleware with default options
	r.Use(mw.RequestID)
	r.Use(mw.LoggerWithOptions(nil))
	r.Use(mw.CORSWithOptions(nil))

	// route group for API v1
	apiv1 := r.Group("/api/v1")

	// OIDC middleware for authentication
	oidcConfig := mw.OIDCProviderConfig{
		ClientID:     os.Getenv("PGO_OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("PGO_OIDC_CLIENT_SECRET"),
		Issuer:       os.Getenv("PGO_OIDC_ISSUER"),
	}
	apiv1.Use(mw.VerifyOIDCToken(oidcConfig))

	// Postgres configuration for authorization
	pgConfig := mw.PgConfig{
		ConnString: os.Getenv("PGO_POSTGRES_CONN_STRING"),
	}
	pgmw := mw.Postgres(pgConfig,
		mw.PgOIDCAuthz(oidcConfig, os.Getenv("PGO_POSTGRES_OIDC_ROLE_CLAIM_KEY")),
	)
	apiv1.Use(pgmw)

	// Respond with all networks where user_id == authenticated user.Subject
	apiv1.Handle("GET /networks", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := `SELECT id, name, addr, addr6, dns, user_id, info, domains, created_at,
		updated_at, uuid FROM networks WHERE user_id = $1`

		user, _ := pgo.OIDCUser(r)
		pgo.SelectAndRespondJSON[wg.Network](w, r, query, []any{user.Subject}, pgx.RowToStructByPos[wg.Network])
	}))

	apiv1.Handle("POST /networks", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqNet wg.Network
		if err := pgo.BindOrRespondError(r, w, &reqNet); err != nil {
			return
		}

		user, _ := pgo.OIDCUser(r)
		reqNet.UserID = user.Subject
		network := wg.NewDefaultNetwork(reqNet)
		networkMap := pgo.RowMap(network)

		if _, pgErr := pgo.InsertRow(r, "networks", networkMap); pgErr != nil {
			pgo.RespondError(w, pgo.PgErrorCodeToHTTPStatus(pgErr.Error()), pgErr.Error())
			return
		}
		pgo.RespondJSON(w, http.StatusCreated, network)
	}))

	// Respond with all peers created by the authenticated user and network_id == network path parameter
	apiv1.Handle("GET /networks/{network}/peers", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := `SELECT id, name, addr, cidr, dns, network_id, allowed_ips, endpoint,
		enabled, type, pubkey, privkey, presharedkey, info, user_id, listen_port, mtu, created_at, updated_at,
		uuid FROM peers WHERE network_id = $1 AND user_id = $2`

		user, _ := pgo.OIDCUser(r)
		pgo.SelectAndRespondJSON[wg.Peer](w, r, query, []any{r.PathValue("network"), user.Subject}, pgx.RowToStructByPos[wg.Peer])
	}))

	apiv1.Handle("POST /networks/{network}/peers", http.HandlerFunc(postPeerHandler))
	apiv1.Handle("GET /networks/{network}/peers/{peer}", http.HandlerFunc(getPeerConfigHandler))
	apiv1.Handle("GET /networks/{network}/peers/{peer}/qr", http.HandlerFunc(getPeerConfigQrHandler))
	apiv1.Handle("DELETE /networks/{network}", http.HandlerFunc(deleteNetworkHandler))
	apiv1.Handle("DELETE /networks/{network}/peers/{peer}", http.HandlerFunc(deletePeerHandler))

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

## Contributing
Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## License
Apache License 2.0


