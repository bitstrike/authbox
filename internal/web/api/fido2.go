package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	// Basic format validation (pamu2fcfg output contains colons and commas)
	if !strings.Contains(body.CredentialData, ",") {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "credential_data format invalid (expected pamu2fcfg output)")
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
