package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/ca"
	"github.com/authbox/authbox/internal/db"
	"github.com/authbox/authbox/internal/ldap"
	"github.com/go-chi/chi/v5"
)

type API struct {
	ldap       *ldap.Client
	ca         *ca.CA
	repo       *db.Repository
	sshCertTTL string
}

func New(ldapClient *ldap.Client, sshCA *ca.CA, repo *db.Repository, sshCertTTL string) *API {
	return &API{
		ldap:       ldapClient,
		ca:         sshCA,
		repo:       repo,
		sshCertTTL: sshCertTTL,
	}
}

func RegisterRoutes(r chi.Router) {
	// Placeholder - called from main.go before API is fully wired.
	// Use RegisterRoutesWithDeps for full setup.
}

func (a *API) RegisterRoutesWithDeps(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1", func(r chi.Router) {
		// Unauthenticated
		r.Get("/ssh/ca.pub", a.getCAPublicKey)

		// Authenticated endpoints
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			// Users
			r.With(auth.RequireRole(auth.RoleViewer)).Get("/users", a.listUsers)
			r.With(auth.RequireRole(auth.RoleOperator)).Post("/users", a.createUser)
			r.With(auth.RequireRole(auth.RoleOperator)).Put("/users/{uid}", a.updateUser)
			r.With(auth.RequireRole(auth.RoleOperator)).Post("/users/{uid}/disable", a.disableUser)
			r.With(auth.RequireRole(auth.RoleAdmin)).Post("/users/{uid}/enable", a.enableUser)
			r.With(auth.RequireRole(auth.RoleOperator)).Post("/users/import", a.importUsers)

			// Groups
			r.With(auth.RequireRole(auth.RoleViewer)).Get("/groups", a.listGroups)
			r.With(auth.RequireRole(auth.RoleAdmin)).Post("/groups", a.createGroup)
			r.With(auth.RequireRole(auth.RoleOperator)).Put("/groups/{cn}", a.updateGroup)
			r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/groups/{cn}", a.deleteGroup)

			// SSH
			r.With(auth.RequireRole(auth.RoleSelf)).Post("/ssh/sign", a.signSSHKey)
			r.With(auth.RequireRole(auth.RoleViewer)).Get("/ssh/certs", a.listSSHCerts)

			// FIDO2
			r.With(auth.RequireRole(auth.RoleSelf)).Post("/fido2/register", a.registerFIDO2)
			r.With(auth.RequireRole(auth.RoleSelf)).Get("/fido2/credentials/{uid}", a.getFIDO2Credentials)
			r.With(auth.RequireRole(auth.RoleViewer)).Get("/fido2/credentials", a.getAllFIDO2Credentials)
			r.With(auth.RequireRole(auth.RoleSelf)).Delete("/fido2/credentials/{id}", a.deleteFIDO2Credential)

			// Config
			r.With(auth.RequireRole(auth.RoleAdmin)).Get("/config/export", a.exportConfig)
			r.With(auth.RequireRole(auth.RoleAdmin)).Post("/config/import", a.importConfig)
		})
	})

	r.Route("/internal", func(r chi.Router) {
		// System role required - added in Phase 10
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, code string, message string) {
	respondJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func respondList(w http.ResponseWriter, data any, offset, limit, total int) {
	respondJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

func paginationParams(r *http.Request) (int, int) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return offset, limit
}
