// devmode.go provides development mode support when OIDC is not configured.
// Auto-creates an admin session for any request, granting all roles without
// requiring authentication. Used for local development and testing.
package frontend

import (
	"net/http"

	"github.com/authbox/authbox/internal/auth"
)

// NewFrontendDevMode creates a frontend that auto-authenticates as admin.
// Used when OIDC is not configured (development/testing).
func NewFrontendDevMode(sessions *auth.SessionStore, deps *Deps) *Frontend {
	return &Frontend{
		sessions: sessions,
		authH:    nil,
		h:        newHandlers(deps),
	}
}

// RegisterRoutes for dev mode skips OIDC auth routes and auto-creates a session.
func (f *Frontend) RegisterRoutesDevMode(r interface{ Route(string, func(r interface{})) }) {
	// This is handled by overriding requireSession in dev mode
}

// devModeSession is middleware that auto-creates an admin session.
func (f *Frontend) devModeSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := f.sessions.GetFromRequest(r)
		if sess == nil {
			// Auto-create session as dev admin
			sessionID := f.sessions.Create("admin@localhost")
			f.sessions.SetCookie(w, sessionID)
			sess = f.sessions.Get(sessionID)
		}

		ctx := auth.SetClaims(r.Context(), &auth.Claims{Email: sess.Email})
		// Grant all roles in dev mode
		roles := []auth.Role{auth.RoleSelf, auth.RoleViewer, auth.RoleOperator, auth.RoleAdmin}
		ctx = auth.SetRoles(ctx, roles)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
