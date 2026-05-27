// router.go defines the Frontend struct and registers all web UI routes on the
// chi router. Handles static file serving, OIDC login/logout routes, session
// middleware, and role-gated route groups (viewer, operator, admin). This is
// the entry point for wiring the frontend into the application server.
package frontend

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/authbox/authbox/internal/auth"
	"github.com/go-chi/chi/v5"
)

//go:embed static/*
var staticFS embed.FS

type Frontend struct {
	sessions *auth.SessionStore
	authH    *AuthHandlers
	h        *handlers
}

func NewFrontend(sessions *auth.SessionStore, authHandlers *AuthHandlers, deps *Deps) *Frontend {
	return &Frontend{
		sessions: sessions,
		authH:    authHandlers,
		h:        newHandlers(deps),
	}
}

func (f *Frontend) RegisterRoutes(r chi.Router) {
	// Static files
	staticContent, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	// Auth routes (only when OIDC is configured)
	if f.authH != nil {
		r.Get("/login", f.authH.HandleLogin)
		r.Get("/auth/callback", f.authH.HandleCallback)
		r.Post("/logout", f.authH.HandleLogout)
	} else {
		// Dev mode: login redirects to dashboard
		r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/", http.StatusFound)
		})
		r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/", http.StatusFound)
		})
	}

	// Session middleware
	sessionMiddleware := f.requireSession
	if f.authH == nil {
		sessionMiddleware = f.devModeSession
	}

	// Protected routes (require session)
	r.Group(func(r chi.Router) {
		r.Use(sessionMiddleware)

		r.Get("/", f.h.dashboard)
		r.Get("/status", f.h.status)

		// HTMX partials
		f.registerPartials(r)

		// Form actions
		f.registerActions(r)

		// SSH - available to all authenticated users (self role)
		r.Get("/ssh", f.h.ssh)
		r.Get("/fido2", f.h.fido2)

		// Viewer+ routes
		r.Group(func(r chi.Router) {
			r.Use(requireFrontendRole(auth.RoleViewer))
			r.Get("/users", f.h.users)
			r.Get("/groups", f.h.groups)
		})

		// Operator+ routes
		r.Group(func(r chi.Router) {
			r.Use(requireFrontendRole(auth.RoleOperator))
			r.Get("/users/new", f.h.userNew)
			r.Get("/users/{uid}/edit", f.h.userEdit)
			r.Get("/users/import", f.h.userImport)
		})

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(requireFrontendRole(auth.RoleAdmin))
			r.Get("/groups/new", f.h.groupNew)
			r.Get("/groups/{cn}/edit", f.h.groupEdit)
			r.Get("/service-accounts", f.h.serviceAccounts)
			r.Get("/logs", f.h.logs)
			r.Get("/settings", f.h.settings)
			r.Get("/backup", f.h.backup)
		})
	})
}

func (f *Frontend) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := f.sessions.GetFromRequest(r)
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		// Set claims in context
		ctx := auth.SetClaims(r.Context(), &auth.Claims{Email: sess.Email})
		// Look up roles
		if f.h.deps.Roles != nil {
			roles, err := f.h.deps.Roles.GetRolesForUser(sess.Email)
			if err == nil {
				roles = append(roles, auth.RoleSelf)
				ctx = auth.SetRoles(ctx, roles)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireFrontendRole(role auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !auth.HasRole(r.Context(), role) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RegisterRoutesLegacy is the fallback when OIDC is not configured.
func RegisterRoutesLegacy(r chi.Router) {
	staticContent, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><h1>Authbox</h1><p>OIDC not configured</p></body></html>"))
	})
}
