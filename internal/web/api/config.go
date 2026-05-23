package api

import "net/http"

func (a *API) exportConfig(w http.ResponseWriter, r *http.Request) {
	// TODO: implement full export (LDAP directory, cn=config, FIDO2 mappings, SQLite) in Phase 9
	respondError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "export not yet available")
}

func (a *API) importConfig(w http.ResponseWriter, r *http.Request) {
	// TODO: implement import in Phase 9
	respondError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "import not yet available")
}
