// ssh.go implements the REST API handlers for SSH certificate operations:
// serving the CA public key (unauthenticated), signing user public keys with
// the CA, and listing issued certificates for audit. Validates that the user
// exists and is active before signing, and records each cert in SQLite.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/constants"
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

	// Build principal list: caller's uid first (KeyId/self-login), then any
	// SSH login role principals from sshrole-* group membership.
	principals := []string{principal}
	roles, err := a.ldap.GetSSHRolesForUser(principal)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to resolve ssh roles")
		return
	}
	principals = append(principals, roles...)

	// Use configured TTL
	ttlSeconds := a.certTTLSeconds()

	// Generate serial (use UnixNano for uniqueness)
	serial := uint64(time.Now().UnixNano())

	certBytes, err := a.ca.SignPublicKey([]byte(body.PublicKey), principals, ttlSeconds, serial)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "failed to sign key: "+err.Error())
		return
	}

	// Record the issued cert. Principal stores the full comma-joined list so the
	// valid-serials cache and host cert-check can authorize role logins.
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	a.repo.CreateSSHCert(&db.SSHCert{
		Username:  principal,
		Serial:    fmt.Sprintf("%d", serial),
		Principal: strings.Join(principals, ","),
		ExpiresAt: expiresAt,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"certificate": string(certBytes),
		"principal":   principal,
		"principals":  principals,
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
		return constants.DefaultSSHCertTTLSeconds
	}
	d, err := time.ParseDuration(a.sshCertTTL)
	if err != nil {
		return constants.DefaultSSHCertTTLSeconds
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

func (a *API) validSerials(w http.ResponseWriter, r *http.Request) {
	certs, err := a.repo.ListValidSSHCerts()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	// c.Principal is the comma-joined principal list (e.g. "alice,ops"), so each
	// line is "serial:alice,ops". The host cert-check script splits on the first
	// colon, then on commas. Principal charset [a-z0-9-] never collides with ','.
	for _, c := range certs {
		fmt.Fprintf(w, "%s:%s\n", c.Serial, c.Principal)
	}
}
