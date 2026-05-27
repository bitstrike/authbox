// groups.go implements the REST API handlers for group management: list,
// create, update membership, and delete. Supports both posixGroup and
// groupOfNames types with GID uniqueness validation.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/authbox/authbox/internal/ldap"
	"github.com/go-chi/chi/v5"
)

func (a *API) listGroups(w http.ResponseWriter, r *http.Request) {
	offset, limit := paginationParams(r)
	groups, total, err := a.ldap.ListGroups(offset, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to list groups")
		return
	}
	respondList(w, groups, offset, limit, total)
}

func (a *API) createGroup(w http.ResponseWriter, r *http.Request) {
	var g ldap.Group
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if g.CN == "" {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "cn is required")
		return
	}
	if g.Type != "posixGroup" && g.Type != "groupOfNames" {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "type must be posixGroup or groupOfNames")
		return
	}

	// Check GID uniqueness for posixGroup
	if g.Type == "posixGroup" && g.GIDNumber > 0 {
		exists, err := a.ldap.GIDExists(g.GIDNumber)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "INTERNAL", "gid check failed")
			return
		}
		if exists {
			respondError(w, http.StatusConflict, "CONFLICT", "gidNumber already in use")
			return
		}
	}

	if err := a.ldap.CreateGroup(&g); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to create group")
		return
	}

	respondJSON(w, http.StatusCreated, g)
}

func (a *API) updateGroup(w http.ResponseWriter, r *http.Request) {
	cn := chi.URLParam(r, "cn")

	var body struct {
		Members []string `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if err := a.ldap.UpdateGroupMembers(cn, body.Members); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to update group")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *API) deleteGroup(w http.ResponseWriter, r *http.Request) {
	cn := chi.URLParam(r, "cn")

	if err := a.ldap.DeleteGroup(cn); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to delete group")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
