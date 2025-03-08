package pgo

import (
	"cmp"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edgeflare/pgo/pkg/config"
	"github.com/edgeflare/pgo/pkg/httputil"
	mw "github.com/edgeflare/pgo/pkg/httputil/middleware"
	"github.com/edgeflare/pgo/pkg/rest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var restCmd = &cobra.Command{
	Use:   "rest",
	Short: "Start the REST API server",
	Long:  `Starts a REST API server that provides access to PostgreSQL data through HTTP endpoints`,
	Run:   runRESTServer,
}

func init() {
	defaults := config.DefaultRESTConfig()
	f := restCmd.Flags()
	f.StringP("rest.pg.connString", "c", "", "PostgreSQL connection string")
	f.StringP("rest.listenAddr", "l", defaults.ListenAddr, "REST server listen address")
	f.String("rest.baseURL", "", "Base URL for API endpoints")
	f.Bool("rest.oidc.enabled", false, "Enable OIDC authentication")
	f.String("rest.oidc.clientID", "", "OIDC client ID")
	f.String("rest.oidc.clientSecret", "", "OIDC client secret")
	f.String("rest.oidc.issuer", "", "OIDC issuer URL")
	f.String("rest.oidc.roleClaimKey", defaults.OIDC.RoleClaimKey, "JWT claim path for PostgreSQL role")
	f.Bool("rest.basicAuthEnabled", false, "Enable basic authentication")
	f.Bool("rest.anonRoleEnabled", defaults.AnonRoleEnabled, "Enable anonymous role")

	viper.BindPFlags(f)
	rootCmd.AddCommand(restCmd)
}

func runRESTServer(cmd *cobra.Command, args []string) {
	connString := viper.GetString("rest.pg.connString")
	if connString == "" {
		connString = os.Getenv("PGO_REST_PG_CONN_STRING")
		if connString == "" {
			log.Fatal("PostgreSQL connection string required")
		}
	}

	oidcConfig := &mw.OIDCProviderConfig{
		ClientID:     cmp.Or(os.Getenv("PGO_OIDC_CLIENT_ID"), viper.GetString("rest.oidc.clientID")),
		ClientSecret: cmp.Or(os.Getenv("PGO_OIDC_CLIENT_SECRET"), viper.GetString("rest.oidc.clientSecret")),
		Issuer:       cmp.Or(os.Getenv("PGO_OIDC_ISSUER"), viper.GetString("rest.oidc.issuer")),
	}

	roleClaimKey := cmp.Or(
		os.Getenv("PGO_POSTGRES_OIDC_ROLE_CLAIM_KEY"),
		viper.GetString("rest.oidc.roleClaimKey"),
		".policies.pgrole",
	)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}

	// Create and configure server
	server, err := rest.NewServer(connString, viper.GetString("rest.baseURL"))
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Set middlewares
	server.SetMiddlewares([]httputil.Middleware{
		mw.RequestID,
		mw.LoggerWithOptions(nil),
		mw.CORSWithOptions(nil),
		mw.VerifyOIDCToken(*oidcConfig),
		mw.Postgres(pool, mw.PgOIDCAuthz(*oidcConfig, roleClaimKey)),
	})

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.Start(viper.GetString("rest.listenAddr")); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server gracefully stopped")
}
