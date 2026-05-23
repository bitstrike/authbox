package auth

import (
	"context"
	"net/http"
)

type Role string

const (
	RoleSelf     Role = "self"
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
	RoleSystem   Role = "system"
)

type contextKey string

const claimsKey contextKey = "claims"
const rolesKey contextKey = "roles"

func SetClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func GetClaims(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

func SetRoles(ctx context.Context, roles []Role) context.Context {
	return context.WithValue(ctx, rolesKey, roles)
}

func GetRoles(ctx context.Context) []Role {
	r, _ := ctx.Value(rolesKey).([]Role)
	return r
}

func HasRole(ctx context.Context, required Role) bool {
	roles := GetRoles(ctx)
	for _, r := range roles {
		if r == required || r == RoleAdmin {
			return true
		}
	}
	return false
}

// RequireRole returns middleware that rejects requests without the specified role.
func RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !HasRole(r.Context(), role) {
				http.Error(w, `{"error":{"code":"FORBIDDEN","message":"insufficient permissions"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
