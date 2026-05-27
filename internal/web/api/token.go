// token.go implements the OAuth2 client credentials grant for service
// accounts. Validates client_id/client_secret against bcrypt hashes in SQLite,
// issues opaque bearer tokens stored in memory with a 1-hour TTL, and provides
// ValidateServiceToken for the auth middleware to verify API requests.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var serviceTokens sync.Map

type serviceTokenEntry struct {
	ClientID  string
	Role      string
	ExpiresAt time.Time
}

// ValidateServiceToken checks if a token is valid and returns the role.
func ValidateServiceToken(token string) (string, string, bool) {
	val, ok := serviceTokens.Load(token)
	if !ok {
		return "", "", false
	}
	entry := val.(*serviceTokenEntry)
	if time.Now().After(entry.ExpiresAt) {
		serviceTokens.Delete(token)
		return "", "", false
	}
	return entry.ClientID, entry.Role, true
}

// tokenHandler handles the OAuth2 client credentials grant.
// POST /oauth/token with client_id and client_secret in form body.
func (a *API) tokenHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid form data")
		return
	}

	grantType := r.FormValue("grant_type")
	if grantType != "client_credentials" {
		respondError(w, http.StatusBadRequest, "UNSUPPORTED_GRANT", "only client_credentials grant is supported")
		return
	}

	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")

	if clientID == "" || clientSecret == "" {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "client_id and client_secret are required")
		return
	}

	// Look up service account
	sa, err := a.repo.GetServiceAccountByClientID(clientID)
	if err != nil || sa == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
		return
	}

	// Verify secret
	if err := bcrypt.CompareHashAndPassword([]byte(sa.ClientSecretHash), []byte(clientSecret)); err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials")
		return
	}

	// Update last used
	a.repo.UpdateServiceAccountLastUsed(clientID)

	// Issue a simple signed token (in production, use JWT)
	// For now, return a token that encodes the client_id and role
	token := generateServiceToken(clientID, sa.Role)

	respondJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   3600,
		"role":         sa.Role,
	})
}

// generateServiceToken creates a simple bearer token for service accounts.
// In production this would be a signed JWT. For now it's an opaque token
// stored in memory or validated against the DB.
func generateServiceToken(clientID, role string) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	// Store token -> (clientID, role, expiry) mapping
	serviceTokens.Store(token, &serviceTokenEntry{
		ClientID:  clientID,
		Role:      role,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	return token
}
