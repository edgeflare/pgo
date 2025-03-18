package middleware

import (
	"context"

	"github.com/edgeflare/pgo/pkg/httputil"
	"github.com/edgeflare/pgo/pkg/util"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// AuthzResponse contains authorization result.
type AuthzResponse struct {
	Role    string `json:"role"`
	Allowed bool   `json:"allowed"`
}

// AuthzFunc evaluates context and returns authorization status.
type AuthzFunc func(ctx context.Context) (AuthzResponse, error)

// WithOIDCAuthz extracts role from OIDC token and adds to context.
func WithOIDCAuthz(oidcCfg OIDCProviderConfig, roleClaimKey string) AuthzFunc {
	oidcInitOnce.Do(func() {
		if oidcProvider == nil {
			oidcProvider = initOIDCProvider(oidcCfg)
		}
	})
	return func(ctx context.Context) (AuthzResponse, error) {
		user, ok := ctx.Value(httputil.OIDCUserCtxKey).(*oidc.IntrospectionResponse)
		if !ok {
			return AuthzResponse{Allowed: false}, nil
		}
		pgrole, err := util.Jq(user.Claims, roleClaimKey)
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

// WithBasicAuthz creates auth function for Basic Auth.
func WithBasicAuthz() AuthzFunc {
	return func(ctx context.Context) (AuthzResponse, error) {
		user, ok := ctx.Value(httputil.BasicAuthCtxKey).(string)
		if !ok {
			return AuthzResponse{Allowed: false}, nil
		}
		_ = context.WithValue(ctx, httputil.PgRoleCtxKey, user)
		return AuthzResponse{Role: user, Allowed: true}, nil
	}
}

// WithAnonAuthz creates auth function using specified role.
func WithAnonAuthz(anonRole string) AuthzFunc {
	return func(ctx context.Context) (AuthzResponse, error) {
		_ = context.WithValue(ctx, httputil.PgRoleCtxKey, anonRole)
		return AuthzResponse{Role: anonRole, Allowed: true}, nil
	}
}
