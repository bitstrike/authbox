// sync.go implements the internal replication endpoints used by replica
// containers to synchronize SQLite state from the primary. Provides current
// state version, incremental changes since a version, and full snapshot for
// initial sync or recovery. Authenticated via shared secret (system role).
package api

import (
	"net/http"
	"strconv"
)

// syncState returns the current sync version.
// GET /internal/sync/state
func (a *API) syncState(w http.ResponseWriter, r *http.Request) {
	state, err := a.repo.GetSyncState()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, state)
}

// syncChanges returns changes since a given version.
// GET /internal/sync/changes?since={version}
func (a *API) syncChanges(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	since, err := strconv.Atoi(sinceStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_PARAM", "since must be an integer")
		return
	}

	entries, err := a.repo.GetChangesSince(since)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"changes": entries,
		"count":   len(entries),
	})
}

// syncSnapshot returns the full SQLite state for initial sync or recovery.
// GET /internal/sync/snapshot
func (a *API) syncSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := a.repo.GetSnapshot()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, snapshot)
}
