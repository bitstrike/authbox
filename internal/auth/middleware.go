// middleware.go provides HTTP middleware that validates bearer tokens on API
// requests. Tries OIDC verification first, falls back to service account token
// validation. Extracts claims, resolves roles, and populates the request context.
package auth

import (
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
)

// RoleLookup resolves a user's email to their platform roles.
type RoleLookup interface {
	GetRolesForUser(email string) ([]Role, error)
}

// ServiceTokenValidator checks if a token is a valid service account token.
// Returns (clientID, role, valid).
type ServiceTokenValidator func(token string) (string, string, bool)

// TokenMiddleware validates bearer tokens and populates context with claims and roles.
// Tries OIDC verification first. If that fails, falls back to service account token validation.
func TokenMiddleware(verifier *oidc.IDTokenVerifier, roles RoleLookup, svcValidator ServiceTokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ExtractBearerToken(r)
			if token == "" {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing bearer token"}}`, http.StatusUnauthorized)
				return
			}

			// Try OIDC verification first
			idToken, err := verifier.Verify(r.Context(), token)
			if err == nil {
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
				userRoles = append(userRoles, RoleSelf)
				ctx = SetRoles(ctx, userRoles)

				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fallback: try service account token
			if svcValidator != nil {
				clientID, role, valid := svcValidator(token)
				if valid {
					ctx := SetClaims(r.Context(), &Claims{Email: clientID, Sub: clientID})
					var svcRoles []Role
					switch role {
					case "admin":
						svcRoles = []Role{RoleAdmin, RoleOperator, RoleViewer, RoleSelf}
					case "operator":
						svcRoles = []Role{RoleOperator, RoleViewer, RoleSelf}
					case "viewer":
						svcRoles = []Role{RoleViewer, RoleSelf}
					default:
						svcRoles = []Role{RoleSelf}
					}
					ctx = SetRoles(ctx, svcRoles)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid token"}}`, http.StatusUnauthorized)
		})
	}
}
