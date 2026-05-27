// middleware.go provides HTTP middleware that validates OIDC bearer tokens on
// API requests, extracts email claims, resolves the user's roles via LDAP
// group membership, and populates the request context with claims and roles.
package auth

import (
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
)

// RoleLookup resolves a user's email to their platform roles.
type RoleLookup interface {
	GetRolesForUser(email string) ([]Role, error)
}

// TokenMiddleware validates bearer tokens and populates context with claims and roles.
func TokenMiddleware(verifier *oidc.IDTokenVerifier, roles RoleLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ExtractBearerToken(r)
			if token == "" {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing bearer token"}}`, http.StatusUnauthorized)
				return
			}

			idToken, err := verifier.Verify(r.Context(), token)
			if err != nil {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid token"}}`, http.StatusUnauthorized)
				return
			}

			var claims struct {
				Email string `json:"email"`
				Sub   string `json:"sub"`
			}
			if err := idToken.Claims(&claims); err != nil {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid claims"}}`, http.StatusUnauthorized)
				return
			}

			ctx := SetClaims(r.Context(), &Claims{Email: claims.Email, Sub: claims.Sub})

			userRoles, err := roles.GetRolesForUser(claims.Email)
			if err != nil {
				http.Error(w, `{"error":{"code":"INTERNAL","message":"role lookup failed"}}`, http.StatusInternalServerError)
				return
			}
			// All authenticated users get self role
			userRoles = append(userRoles, RoleSelf)
			ctx = SetRoles(ctx, userRoles)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
