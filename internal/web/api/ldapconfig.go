package api

import (
	"encoding/json"
	"net/http"

	"github.com/authbox/authbox/internal/ldap"
)

// getLDAPConfig returns the current cn=config state.
// GET /api/v1/config/ldap
func (a *API) getLDAPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := a.ldap.GetConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

// updateACLs replaces the LDAP ACL list.
// PUT /api/v1/config/ldap/acls
func (a *API) updateACLs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ACLs []string `json:"acls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if len(req.ACLs) == 0 {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", "acls cannot be empty")
		return
	}
	if err := a.ldap.SetACLs(req.ACLs); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "ACLs updated"})
}

// configureReplication sets up syncrepl on this node.
// PUT /api/v1/config/ldap/replication
func (a *API) configureReplication(w http.ResponseWriter, r *http.Request) {
	var req ldap.SyncReplConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if req.Provider == "" || req.SearchBase == "" {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", "provider and search_base required")
		return
	}
	if req.RID == 0 {
		req.RID = 1
	}
	if req.Type == "" {
		req.Type = "refreshAndPersist"
	}

	// Enable syncprov on provider side (idempotent)
	a.ldap.EnableSyncProv()

	if err := a.ldap.ConfigureSyncRepl(&req); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "replication configured"})
}

// removeReplication removes syncrepl configuration.
// DELETE /api/v1/config/ldap/replication
func (a *API) removeReplication(w http.ResponseWriter, r *http.Request) {
	if err := a.ldap.RemoveSyncRepl(); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "replication removed"})
}
