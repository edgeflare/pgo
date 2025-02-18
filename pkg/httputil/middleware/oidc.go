package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/edgeflare/pgo/pkg/httputil"
	"github.com/zitadel/oidc/v3/pkg/client/rs"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// OIDCProvider is the main OIDC provider
type OIDCProvider struct {
	provider rs.ResourceServer
	cache    *Cache
	config   OIDCProviderConfig
	// mu       sync.RWMutex
}

// OIDCProviderConfig holds the configuration for the OIDC provider
type OIDCProviderConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Issuer       string `json:"issuer"`
}

var (
	oidcProvider *OIDCProvider
	oidcInitOnce sync.Once
)

// VerifyOIDCToken is middleware that verifies OIDC tokens in Authorization headers.
// By default, it sends a 401 Unauthorized response if the token is missing or invalid.
// If send401Unauthorized is false, it allows requests with other authorization schemes
// (e.g., Basic Auth) to continue without interference.
func VerifyOIDCToken(oidcCfg OIDCProviderConfig, send401Unauthorized ...bool) func(http.Handler) http.Handler {
	send401 := true // Default behavior: Send 401 on failure
	if len(send401Unauthorized) > 0 {
		send401 = send401Unauthorized[0]
	}

	return func(next http.Handler) http.Handler {
		oidcInitOnce.Do(func() {
			if oidcProvider == nil {
				oidcProvider = InitOIDCProvider(oidcCfg)
			}
		})

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				if send401 {
					http.Error(w, "Authorization header missing", http.StatusUnauthorized)
					return
				}
				// No Authorization header and send401Unauthorized is false,
				// so let other middleware/handlers handle it
				next.ServeHTTP(w, r)
				return
			}

			// Check for "Bearer" token (case-insensitive)
			if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				if send401 {
					http.Error(w, "Invalid token format", http.StatusUnauthorized)
					return
				}
				// Other authorization scheme present and send401Unauthorized is false
				next.ServeHTTP(w, r)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			user, err := rs.Introspect[*oidc.IntrospectionResponse](r.Context(), oidcProvider.provider, tokenString)
			if err != nil || user == nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), httputil.OIDCUserCtxKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func InitOIDCProvider(cfg OIDCProviderConfig) *OIDCProvider {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.Issuer == "" {
		panic("missing required OIDC configuration")
	}

	provider, err := rs.NewResourceServerClientCredentials(context.Background(), cfg.Issuer, cfg.ClientID, cfg.ClientSecret)
	if err != nil {
		log.Fatalf("Failed to create OIDC provider: %v", err)
	}

	return &OIDCProvider{
		config:   cfg,
		provider: provider,
		cache:    NewCache(),
	}
}
