package api

import (
	"encoding/json"
	"net/http"

	"github.com/authbox/authbox/internal/auth"
)

func (a *API) getCAPublicKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write(a.ca.PublicKey())
}

func (a *API) signSSHKey(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "no claims")
		return
	}

	var body struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PublicKey == "" {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "public_key is required")
		return
	}

	// Derive principal from email
	principal := emailToUID(claims.Email)

	// Verify user exists and is active
	user, err := a.ldap.GetUser(principal)
	if err != nil || user == nil {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "user not found in directory")
		return
	}
	if user.Disabled {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "user account is disabled")
		return
	}

	// Sign the key (TTL configured via env, default 12h = 43200s)
	certBytes, err := a.ca.SignPublicKey([]byte(body.PublicKey), principal, 43200)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "failed to sign key: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"certificate": string(certBytes),
		"principal":   principal,
	})
}

func (a *API) listSSHCerts(w http.ResponseWriter, r *http.Request) {
	// TODO: query ssh_certs table with pagination
	respondError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "not yet available")
}

func emailToUID(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}
