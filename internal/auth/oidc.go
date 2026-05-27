// oidc.go sets up the OIDC provider (Google or Entra) using go-oidc, configures
// the OAuth2 client for the authorization code flow, and provides token
// verification and bearer token extraction from HTTP requests.
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCAuth struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    oauth2.Config
}

type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func NewOIDCAuth(ctx context.Context, cfg OIDCConfig) (*OIDCAuth, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, err
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	oauth := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}

	return &OIDCAuth{
		provider: provider,
		verifier: verifier,
		oauth:    oauth,
	}, nil
}

func (o *OIDCAuth) Verifier() *oidc.IDTokenVerifier {
	return o.verifier
}

func (o *OIDCAuth) OAuthConfig() *oauth2.Config {
	return &o.oauth
}

// ExtractBearerToken pulls the token from the Authorization header.
func ExtractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// Claims holds the identity extracted from a verified token.
type Claims struct {
	Email string
	Sub   string
}
