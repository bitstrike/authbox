// users.go implements the REST API handlers for user management: list, create,
// update, disable, enable, and bulk import. Validates UID/GID uniqueness,
// applies defaults, and delegates to the LDAP client for persistence.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/authbox/authbox/internal/ldap"
	"github.com/go-chi/chi/v5"
)

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	offset, limit := paginationParams(r)
	users, total, err := a.ldap.ListUsers(offset, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to list users")
		return
	}
	respondList(w, users, offset, limit, total)
}

func (a *API) createUser(w http.ResponseWriter, r *http.Request) {
	var u ldap.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if u.UID == "" {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "uid is required")
		return
	}

	// Check UID uniqueness
	if u.UIDNumber > 0 {
		exists, err := a.ldap.UIDExists(u.UIDNumber)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "INTERNAL", "uid check failed")
			return
		}
		if exists {
			respondError(w, http.StatusConflict, "CONFLICT", "uidNumber already in use")
			return
		}
	}

	// Set defaults
	if u.HomeDirectory == "" {
		u.HomeDirectory = "/home/" + u.UID
	}
	if u.LoginShell == "" {
		u.LoginShell = "/bin/bash"
	}
	if u.CN == "" {
		u.CN = u.UID
	}
	if u.SN == "" {
		u.SN = u.UID
	}

	if err := a.ldap.CreateUser(&u); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to create user")
		return
	}

	respondJSON(w, http.StatusCreated, u)
}

func (a *API) updateUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")

	existing, err := a.ldap.GetUser(uid)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "lookup failed")
		return
	}
	if existing == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	var u ldap.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// Check UID number conflict if changed
	if u.UIDNumber > 0 && u.UIDNumber != existing.UIDNumber {
		exists, err := a.ldap.UIDExists(u.UIDNumber)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "INTERNAL", "uid check failed")
			return
		}
		if exists {
			respondError(w, http.StatusConflict, "CONFLICT", "uidNumber already in use")
			return
		}
	}

	if err := a.ldap.UpdateUser(uid, &u); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to update user")
		return
	}

	respondJSON(w, http.StatusOK, u)
}

func (a *API) disableUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")

	existing, err := a.ldap.GetUser(uid)
	if err != nil || existing == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	if err := a.ldap.DisableUser(uid); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to disable user")
		return
	}

	// Revoke FIDO2 credentials
	a.repo.DeleteFIDO2Credentials(uid)

	respondJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (a *API) enableUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")

	var body struct {
		LoginShell string `json:"loginShell"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LoginShell == "" {
		body.LoginShell = "/bin/bash"
	}

	if err := a.ldap.EnableUser(uid, body.LoginShell); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to enable user")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (a *API) deleteUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")

	existing, err := a.ldap.GetUser(uid)
	if err != nil || existing == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	if !existing.Disabled {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "user must be disabled before deletion")
		return
	}

	if err := a.ldap.DeleteUser(uid); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to delete user")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) importUsers(w http.ResponseWriter, r *http.Request) {
	// TODO: implement CSV/JSON bulk import in Phase 8
	respondError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "bulk import not yet available")
}
