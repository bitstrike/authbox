// config.go loads application configuration from environment variables and
// secrets files. Reads OIDC credentials (Google or Entra), AWS credentials
// for ACME DNS-01, LDAP admin password, and replica sync secret from the
// runtime secrets directory mounted into the container.
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
	TLSDomain       string
	TLSACMEEmail    string
	AWSHostedZoneID string
	SSHCertTTL      string
	UIDRangeStart   string
	UIDRangeEnd     string
	LogLevel        string
	LogDir          string
	OIDCClientSecret   string
	LDAPAdminPass      string
	InternalSecret     string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
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
		TLSDomain:     getenv("TLS_DOMAIN", ""),
		TLSACMEEmail:  getenv("TLS_ACME_EMAIL", ""),
		AWSHostedZoneID: getenv("AWS_HOSTED_ZONE_ID", ""),
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

	// Google OIDC credentials: /etc/secrets/authbox/google
	// Keys: client_id, client_secret (per Google JSON credential format)
	googleCreds, err := readKeyValueFile(filepath.Join(dir, "google"))
	if err == nil {
		if v, ok := googleCreds["client_id"]; ok {
			c.OIDCClientID = v
		}
		if v, ok := googleCreds["client_secret"]; ok {
			c.OIDCClientSecret = v
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading google secrets: %w", err)
	}

	// Entra ID credentials: /etc/secrets/authbox/entra
	// Keys: AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID
	// Overrides google if both present (only one IdP active).
	entraCreds, err := readKeyValueFile(filepath.Join(dir, "entra"))
	if err == nil {
		if v, ok := entraCreds["azure_client_id"]; ok {
			c.OIDCClientID = v
		}
		if v, ok := entraCreds["azure_client_secret"]; ok {
			c.OIDCClientSecret = v
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading entra secrets: %w", err)
	}

	// AWS credentials: /etc/secrets/authbox/aws
	// Keys: aws_access_key_id, aws_secret_access_key (per ~/.aws/credentials format)
	awsCreds, err := readKeyValueFile(filepath.Join(dir, "aws"))
	if err == nil {
		if v, ok := awsCreds["aws_access_key_id"]; ok {
			c.AWSAccessKeyID = v
		}
		if v, ok := awsCreds["aws_secret_access_key"]; ok {
			c.AWSSecretAccessKey = v
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading aws secrets: %w", err)
	}

	// LDAP admin password: /etc/secrets/authbox/ldap_admin_password
	ldapPass, err := readSecretFile(filepath.Join(dir, "ldap_admin_password"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading ldap_admin_password: %w", err)
	}
	c.LDAPAdminPass = ldapPass

	// Internal shared secret: /etc/secrets/authbox/replica_sync_password
	internalSecret, err := readSecretFile(filepath.Join(dir, "replica_sync_password"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading replica_sync_password: %w", err)
	}
	c.InternalSecret = internalSecret

	return nil
}

func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// readKeyValueFile reads a file with key=value pairs (one per line).
// Lines starting with # are ignored. Empty lines are skipped.
// Key lookup is case-insensitive.
func readKeyValueFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}
	return result, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
