package middleware

import (
	"context"
	"os"

	"github.com/edgeflare/pgo"
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
		user, ok := ctx.Value(pgo.OIDCUserCtxKey).(*oidc.IntrospectionResponse)
		if !ok {
			return AuthzResponse{Allowed: false}, nil
		}

		pgrole, err := util.Jq(user.Claims, pgRoleClaimKey)
		if err != nil {
			return AuthzResponse{Allowed: false}, nil
		}

		ctx = context.WithValue(ctx, pgo.PgRoleCtxKey, pgrole)
		return AuthzResponse{Role: pgrole.(string), Allowed: true}, nil
	}
}

// WithBasicAuthz returns an authorization function for Basic Auth
func PgBasicAuthz() AuthzFunc {
	return func(ctx context.Context) (AuthzResponse, error) {
		user, ok := ctx.Value(pgo.BasicAuthCtxKey).(string)
		if !ok {
			return AuthzResponse{Allowed: false}, nil
		}
		ctx = context.WithValue(ctx, pgo.PgRoleCtxKey, user)
		return AuthzResponse{Role: user, Allowed: true}, nil
	}
}

// WithAnonAuthz returns an authorization function for anonymous users
func PgAnonAuthz() AuthzFunc {
	return func(ctx context.Context) (AuthzResponse, error) {
		pgrole := os.Getenv("PGO_POSTGRES_ANON_ROLE")
		if pgrole == "" {
			return AuthzResponse{Allowed: false}, nil
		}

		// ctx = context.WithValue(ctx, pgo.PgRoleCtxKey, pgrole)
		return AuthzResponse{Role: pgrole, Allowed: true}, nil
	}
}
