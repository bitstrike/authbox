// serviceaccounts.go implements the REST API handlers for service account
// CRUD: list, create (generates client_id and bcrypt-hashed client_secret),
// and delete. The client_secret is returned only once at creation time.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/authbox/authbox/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func (a *API) listServiceAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.repo.ListServiceAccounts()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to list service accounts")
		return
	}
	respondJSON(w, http.StatusOK, accounts)
}

func (a *API) createServiceAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Description string `json:"description"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if body.Role == "" {
		body.Role = "viewer"
	}
	validRoles := map[string]bool{"viewer": true, "operator": true, "admin": true}
	if !validRoles[body.Role] {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "role must be viewer, operator, or admin")
		return
	}

	clientID := generateClientID()
	clientSecret := generateClientSecret()

	hash, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to hash secret")
		return
	}

	err = a.repo.CreateServiceAccount(&db.ServiceAccount{
		ClientID:         clientID,
		ClientSecretHash: string(hash),
		Description:      body.Description,
		Role:             body.Role,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to create service account")
		return
	}

	// Return the secret only once
	respondJSON(w, http.StatusCreated, map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"description":   body.Description,
		"role":          body.Role,
	})
}

func (a *API) deleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ClientID == "" {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "client_id is required")
		return
	}

	if err := a.repo.DeleteServiceAccount(body.ClientID); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to delete service account")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func generateClientID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "sa_" + hex.EncodeToString(b)
}

func generateClientSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
