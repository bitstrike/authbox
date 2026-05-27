// auth.go implements the OIDC authorization code flow for web UI login.
// Handles the login redirect to the IdP, the callback that exchanges the
// authorization code for tokens, ID token verification, session creation,
// and logout. Verifies the user exists in LDAP before granting access.
package frontend

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/authbox/authbox/internal/auth"
	"golang.org/x/oauth2"
)

type AuthHandlers struct {
	oidc     *auth.OIDCAuth
	sessions *auth.SessionStore
	roles    auth.RoleLookup
}

func NewAuthHandlers(oidc *auth.OIDCAuth, sessions *auth.SessionStore, roles auth.RoleLookup) *AuthHandlers {
	return &AuthHandlers{
		oidc:     oidc,
		sessions: sessions,
		roles:    roles,
	}
}

// HandleLogin redirects to the OIDC provider.
func (h *AuthHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   300,
	})

	url := h.oidc.OAuthConfig().AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusFound)
}

// HandleCallback processes the OIDC callback after IdP authentication.
func (h *AuthHandlers) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	oauth2Token, err := h.oidc.OAuthConfig().Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	// Extract ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in response", http.StatusInternalServerError)
		return
	}

	// Verify ID token
	idToken, err := h.oidc.Verifier().Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "token verification failed", http.StatusUnauthorized)
		return
	}

	// Extract claims
	var claims struct {
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "invalid claims", http.StatusInternalServerError)
		return
	}

	// Verify user exists in LDAP (via role lookup which checks LDAP)
	_, err = h.roles.GetRolesForUser(claims.Email)
	if err != nil {
		http.Error(w, "user not found in directory", http.StatusForbidden)
		return
	}

	// Create session
	sessionID := h.sessions.Create(claims.Email)
	h.sessions.SetCookie(w, sessionID)

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleLogout destroys the session.
func (h *AuthHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		h.sessions.Delete(cookie.Value)
	}
	h.sessions.ClearCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
