// fido2.go implements the REST API handlers for FIDO2/U2F credential
// management: registering new credentials (pamu2fcfg output), retrieving
// per-user or all credentials (optionally in pam_u2f mapping format), and
// revoking individual credentials. Enforces self/operator permission checks.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/db"
	"github.com/go-chi/chi/v5"
)

func (a *API) registerFIDO2(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "no claims")
		return
	}

	var body struct {
		UID            string `json:"uid"`
		CredentialData string `json:"credential_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// Self role can only register for themselves
	uid := body.UID
	if uid == "" {
		uid = emailToUID(claims.Email)
	}
	if uid != emailToUID(claims.Email) && !auth.HasRole(r.Context(), auth.RoleOperator) {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "cannot register credentials for another user")
		return
	}

	if body.CredentialData == "" {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "credential_data is required")
		return
	}

	if err := validatePamU2FCredential(body.CredentialData); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	err := a.repo.CreateFIDO2Credential(&db.FIDO2Credential{
		UID:            uid,
		CredentialData: body.CredentialData,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to store credential")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"status": "registered", "uid": uid})
}

func (a *API) getFIDO2Credentials(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	claims := auth.GetClaims(r.Context())

	// Self can only view own credentials
	if uid != emailToUID(claims.Email) && !auth.HasRole(r.Context(), auth.RoleViewer) {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
		return
	}

	creds, err := a.repo.GetFIDO2Credentials(uid)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to get credentials")
		return
	}

	respondJSON(w, http.StatusOK, creds)
}

func (a *API) getAllFIDO2Credentials(w http.ResponseWriter, r *http.Request) {
	creds, err := a.repo.GetAllFIDO2Credentials()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to get credentials")
		return
	}

	// Format as pam_u2f mappings file content
	if r.URL.Query().Get("format") == "pam" {
		w.Header().Set("Content-Type", "text/plain")
		for _, c := range creds {
			fmt.Fprintf(w, "%s:%s\n", c.UID, c.CredentialData)
		}
		return
	}

	respondJSON(w, http.StatusOK, creds)
}

func (a *API) deleteFIDO2Credential(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid credential id")
		return
	}

	claims := auth.GetClaims(r.Context())
	cred, err := a.repo.GetFIDO2CredentialByID(id)
	if err != nil || cred == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "credential not found")
		return
	}

	// Self can only delete own credentials
	if cred.UID != emailToUID(claims.Email) && !auth.HasRole(r.Context(), auth.RoleOperator) {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
		return
	}

	if err := a.repo.DeleteFIDO2CredentialByID(id); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL", "failed to delete credential")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// validatePamU2FCredential checks that the credential data looks like pamu2fcfg output.
// Expected format: <credential_id>,<public_key>,<key_type>,<options>
// Example: ABCdef123...,DEFabc456...,es256,+presence
func validatePamU2FCredential(data string) error {
	data = strings.TrimSpace(data)
	if data == "" {
		return fmt.Errorf("credential_data is empty")
	}

	parts := strings.Split(data, ",")
	if len(parts) < 2 {
		return fmt.Errorf("credential_data format invalid: expected at least credential_id,public_key separated by commas")
	}

	// Credential ID should be a base64-like string (alphanumeric, +, /, =)
	credID := parts[0]
	if len(credID) < 10 {
		return fmt.Errorf("credential_id too short (expected base64url-encoded value)")
	}

	// Public key should also be base64-like
	pubKey := parts[1]
	if len(pubKey) < 10 {
		return fmt.Errorf("public_key too short (expected base64url-encoded value)")
	}

	return nil
}
