package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Role            string
	PrimaryHost     string
	RuntimeSecrets  string
	LDAPBaseDN      string
	OIDCIssuerURL   string
	OIDCClientID    string
	InitialAdmin    string
	TLSCertPath     string
	TLSKeyPath      string
	SSHCertTTL      string
	UIDRangeStart   string
	UIDRangeEnd     string
	LogLevel        string
	LogDir          string
	OIDCClientSecret string
	LDAPAdminPass    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Role:          getenv("ROLE", "primary"),
		PrimaryHost:   getenv("PRIMARY_HOST", ""),
		RuntimeSecrets: getenv("RUNTIME_SECRETS", "/etc/secrets/authbox"),
		LDAPBaseDN:    getenv("LDAP_BASE_DN", "dc=example,dc=com"),
		OIDCIssuerURL: getenv("OIDC_ISSUER_URL", ""),
		OIDCClientID:  getenv("OIDC_CLIENT_ID", ""),
		InitialAdmin:  getenv("INITIAL_ADMIN_EMAIL", ""),
		TLSCertPath:   getenv("TLS_CERT_PATH", "/data/tls/cert.pem"),
		TLSKeyPath:    getenv("TLS_KEY_PATH", "/data/tls/key.pem"),
		SSHCertTTL:    getenv("SSH_CERT_TTL", "12h"),
		UIDRangeStart: getenv("UID_RANGE_START", "10000"),
		UIDRangeEnd:   getenv("UID_RANGE_END", "60000"),
		LogLevel:      getenv("LOG_LEVEL", "info"),
		LogDir:        getenv("LOG_DIR", "/app/logs"),
	}

	if err := cfg.loadSecrets(); err != nil {
		return nil, fmt.Errorf("loading secrets: %w", err)
	}

	return cfg, nil
}

func (c *Config) loadSecrets() error {
	dir := c.RuntimeSecrets
	if dir == "" {
		return nil
	}

	oidcSecret, err := readSecretFile(filepath.Join(dir, "oidc_client_secret"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	c.OIDCClientSecret = oidcSecret

	ldapPass, err := readSecretFile(filepath.Join(dir, "ldap_admin_password"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	c.LDAPAdminPass = ldapPass

	return nil
}

func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
