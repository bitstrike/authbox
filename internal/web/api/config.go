// config.go implements the REST API handlers for backup export and import.
// Export produces a gzipped tar archive containing the LDAP directory, cn=config,
// FIDO2 mappings, and SQLite state. Import stages LDIF files for restore on
// container restart and restores SQLite state immediately.
package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/authbox/authbox/internal/backup"
	"github.com/authbox/authbox/internal/constants"
)

const slapcatPath = "/usr/sbin/slapcat"
const restoreDir = "/data/live-restore"

func (a *API) exportConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		"attachment; filename=authbox-export-%s.tar.gz",
		r.URL.Query().Get("ts"),
	))

	if err := backup.CreateExport(w, a.repo, slapcatPath); err != nil {
		// If we haven't written headers yet this will work; otherwise client gets truncated archive
		respondError(w, http.StatusInternalServerError, "EXPORT_FAILED", err.Error())
	}
}

func (a *API) importConfig(w http.ResponseWriter, r *http.Request) {
	// Confirm header required
	confirm := r.Header.Get("X-Confirm")
	if confirm != constants.ConfirmWord {
		respondError(w, http.StatusBadRequest, "CONFIRMATION_REQUIRED",
			fmt.Sprintf("Set X-Confirm header to %q to proceed", constants.ConfirmWord))
		return
	}

	data, err := backup.ImportExport(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ARCHIVE", err.Error())
		return
	}

	// Stage LDIF files for restore on next startup
	if err := os.MkdirAll(restoreDir, 0750); err != nil {
		respondError(w, http.StatusInternalServerError, "STAGE_FAILED", "failed to create restore directory: "+err.Error())
		return
	}

	if len(data.DirectoryLDIF) > 0 {
		if err := os.WriteFile(restoreDir+"/directory.ldif", data.DirectoryLDIF, 0640); err != nil {
			respondError(w, http.StatusInternalServerError, "STAGE_FAILED", "failed to write directory LDIF: "+err.Error())
			return
		}
	}

	if len(data.ConfigLDIF) > 0 {
		if err := os.WriteFile(restoreDir+"/config.ldif", data.ConfigLDIF, 0640); err != nil {
			respondError(w, http.StatusInternalServerError, "STAGE_FAILED", "failed to write config LDIF: "+err.Error())
			return
		}
	}

	// Restore application state immediately
	if err := backup.RestoreState(a.repo, &data.State); err != nil {
		respondError(w, http.StatusInternalServerError, "STATE_RESTORE_FAILED", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":  "restore staged, container restarting",
		"restart":  true,
		"version":  data.Meta.Version,
		"exported": data.Meta.CreatedAt,
	})

	// Exit to trigger container restart; entrypoint will apply the staged LDIF
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}
