package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/db"
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

	// Use configured TTL
	ttlSeconds := a.certTTLSeconds()

	certBytes, err := a.ca.SignPublicKey([]byte(body.PublicKey), principal, ttlSeconds)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "failed to sign key: "+err.Error())
		return
	}

	// Record the issued cert
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	serial := fmt.Sprintf("%d", time.Now().UnixNano())
	a.repo.CreateSSHCert(&db.SSHCert{
		Username:  principal,
		Serial:    serial,
		Principal: principal,
		ExpiresAt: expiresAt,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"certificate": string(certBytes),
		"principal":   principal,
		"serial":      serial,
		"expires_at":  expiresAt.Format(time.RFC3339),
	})
}

func (a *API) listSSHCerts(w http.ResponseWriter, r *http.Request) {
	offset, limit := paginationParams(r)
	certs, total, err := a.repo.ListSSHCerts(offset, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to list certs")
		return
	}
	respondList(w, certs, offset, limit, total)
}

func (a *API) certTTLSeconds() uint64 {
	if a.sshCertTTL == "" {
		return 43200 // 12h default
	}
	d, err := time.ParseDuration(a.sshCertTTL)
	if err != nil {
		return 43200
	}
	return uint64(d.Seconds())
}

func emailToUID(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}
