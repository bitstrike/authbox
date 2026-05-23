package frontend

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router) {
	r.Get("/", handleIndex)
	r.Get("/login", handleLogin)
	// Remaining pages added in Phase 8
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	// TODO: render dashboard template
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<html><body><h1>Authbox</h1><p>Not yet implemented</p></body></html>"))
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	// TODO: OIDC redirect
	http.Redirect(w, r, "/", http.StatusFound)
}
