package frontend

import (
	"net/http"

	"github.com/authbox/authbox/internal/auth"
	"github.com/go-chi/chi/v5"
)

type Frontend struct {
	sessions *auth.SessionStore
	authH    *AuthHandlers
}

func NewFrontend(sessions *auth.SessionStore, authHandlers *AuthHandlers) *Frontend {
	return &Frontend{
		sessions: sessions,
		authH:    authHandlers,
	}
}

func (f *Frontend) RegisterRoutes(r chi.Router) {
	// Auth routes (unauthenticated)
	r.Get("/login", f.authH.HandleLogin)
	r.Get("/auth/callback", f.authH.HandleCallback)
	r.Post("/logout", f.authH.HandleLogout)

	// Protected routes (require session)
	r.Group(func(r chi.Router) {
		r.Use(f.requireSession)
		r.Get("/", f.handleDashboard)
	})
}

func (f *Frontend) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := f.sessions.GetFromRequest(r)
		if sess == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		// Add claims to context for downstream handlers
		ctx := auth.SetClaims(r.Context(), &auth.Claims{Email: sess.Email})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (f *Frontend) handleDashboard(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<html><body><h1>Authbox</h1><p>Logged in as: " + claims.Email + "</p><form method='POST' action='/logout'><button>Logout</button></form></body></html>"))
}

// RegisterRoutes is the legacy function signature for backward compatibility.
func RegisterRoutes(r chi.Router) {
	// No-op: use NewFrontend().RegisterRoutes() instead when fully wired
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><h1>Authbox</h1><p>Not yet configured</p></body></html>"))
	})
}
