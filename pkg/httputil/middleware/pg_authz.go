package middleware

import (
	"context"
	"os"

	"github.com/edgeflare/pgo/pkg/httputil"
	"github.com/edgeflare/pgo/pkg/util"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// AuthzResponse represents the result of an authorization check
type AuthzResponse struct {
	Role    string `json:"role"`
	Allowed bool   `json:"allowed"`
}

// AuthzFunc defines the function signature for authorization checks
type AuthzFunc func(ctx context.Context) (AuthzResponse, error)

// PgOIDCAuthz is the main authorization function
func PgOIDCAuthz(oidcCfg OIDCProviderConfig, pgRoleClaimKey string) AuthzFunc {
	oidcInitOnce.Do(func() {
		if oidcProvider == nil {
			oidcProvider = InitOIDCProvider(oidcCfg)
		}
	})

	return func(ctx context.Context) (AuthzResponse, error) {
		user, ok := ctx.Value(httputil.OIDCUserCtxKey).(*oidc.IntrospectionResponse)
		if !ok {
			return AuthzResponse{Allowed: false}, nil
		}

		pgrole, err := util.Jq(user.Claims, pgRoleClaimKey)
		if err != nil {
			return AuthzResponse{Allowed: false}, nil
		}

		role, ok := pgrole.(string)
		if !ok {
			return AuthzResponse{Allowed: false}, nil
		}

		_ = context.WithValue(ctx, httputil.PgRoleCtxKey, pgrole)
		return AuthzResponse{Role: role, Allowed: true}, nil
	}
}

// WithBasicAuthz returns an authorization function for Basic Auth
func PgBasicAuthz() AuthzFunc {
	return func(ctx context.Context) (AuthzResponse, error) {
		user, ok := ctx.Value(httputil.BasicAuthCtxKey).(string)
		if !ok {
			return AuthzResponse{Allowed: false}, nil
		}
		_ = context.WithValue(ctx, httputil.PgRoleCtxKey, user)
		return AuthzResponse{Role: user, Allowed: true}, nil
	}
}

// WithAnonAuthz returns an authorization function for anonymous users
func PgAnonAuthz() AuthzFunc {
	return func(_ context.Context) (AuthzResponse, error) {
		pgrole := os.Getenv("PGO_POSTGRES_ANON_ROLE")
		if pgrole == "" {
			return AuthzResponse{Allowed: false}, nil
		}

		// ctx = context.WithValue(ctx, httputil.PgRoleCtxKey, pgrole)
		return AuthzResponse{Role: pgrole, Allowed: true}, nil
	}
}
