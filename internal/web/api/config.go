// config.go implements the REST API handlers for backup export and import.
// Export produces a gzipped tar archive containing the LDAP directory, cn=config,
// FIDO2 mappings, and SQLite state. Import restores from an archive with
// confirmation required via X-Confirm header.
package api

import (
	"fmt"
	"net/http"

	"github.com/authbox/authbox/internal/backup"
	"github.com/authbox/authbox/internal/constants"
)

const slapcatPath = "/usr/sbin/slapcat"
const slapaddPath = "/usr/sbin/slapadd"

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

	// Restore LDAP directory
	if err := backup.RestoreLDAP(slapaddPath, data.DirectoryLDIF, ""); err != nil {
		respondError(w, http.StatusInternalServerError, "LDAP_RESTORE_FAILED", err.Error())
		return
	}

	// Restore cn=config
	if err := backup.RestoreLDAP(slapaddPath, data.ConfigLDIF, "cn=config"); err != nil {
		respondError(w, http.StatusInternalServerError, "CONFIG_RESTORE_FAILED", err.Error())
		return
	}

	// Restore application state
	if err := backup.RestoreState(a.repo, &data.State); err != nil {
		respondError(w, http.StatusInternalServerError, "STATE_RESTORE_FAILED", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":  "import complete",
		"version":  data.Meta.Version,
		"exported": data.Meta.CreatedAt,
	})
}
