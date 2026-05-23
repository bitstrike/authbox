package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/authbox/authbox/internal/config"
)

func TestConfigLoadDefaults(t *testing.T) {
	// Clear any env vars that might interfere
	os.Unsetenv("ROLE")
	os.Unsetenv("LDAP_BASE_DN")

	// Point secrets to a nonexistent dir so file reads are skipped
	os.Setenv("RUNTIME_SECRETS", t.TempDir())
	defer os.Unsetenv("RUNTIME_SECRETS")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Role != "primary" {
		t.Fatalf("expected default role 'primary', got '%s'", cfg.Role)
	}
	if cfg.LDAPBaseDN != "dc=example,dc=com" {
		t.Fatalf("expected default base DN, got '%s'", cfg.LDAPBaseDN)
	}
	if cfg.SSHCertTTL != "12h" {
		t.Fatalf("expected default TTL '12h', got '%s'", cfg.SSHCertTTL)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("expected default log level 'info', got '%s'", cfg.LogLevel)
	}
}

func TestConfigLoadFromEnv(t *testing.T) {
	os.Setenv("ROLE", "replica")
	os.Setenv("PRIMARY_HOST", "primary-host")
	os.Setenv("LDAP_BASE_DN", "dc=test,dc=org")
	os.Setenv("RUNTIME_SECRETS", t.TempDir())
	defer func() {
		os.Unsetenv("ROLE")
		os.Unsetenv("PRIMARY_HOST")
		os.Unsetenv("LDAP_BASE_DN")
		os.Unsetenv("RUNTIME_SECRETS")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Role != "replica" {
		t.Fatalf("expected role 'replica', got '%s'", cfg.Role)
	}
	if cfg.PrimaryHost != "primary-host" {
		t.Fatalf("expected primary host 'primary-host', got '%s'", cfg.PrimaryHost)
	}
	if cfg.LDAPBaseDN != "dc=test,dc=org" {
		t.Fatalf("expected base DN 'dc=test,dc=org', got '%s'", cfg.LDAPBaseDN)
	}
}

func TestConfigLoadSecrets(t *testing.T) {
	dir := t.TempDir()

	// Write secret files
	os.WriteFile(filepath.Join(dir, "oidc_client_secret"), []byte("my-secret\n"), 0600)
	os.WriteFile(filepath.Join(dir, "ldap_admin_password"), []byte("  admin-pass  \n"), 0600)

	os.Setenv("RUNTIME_SECRETS", dir)
	defer os.Unsetenv("RUNTIME_SECRETS")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.OIDCClientSecret != "my-secret" {
		t.Fatalf("expected trimmed secret 'my-secret', got '%s'", cfg.OIDCClientSecret)
	}
	if cfg.LDAPAdminPass != "admin-pass" {
		t.Fatalf("expected trimmed password 'admin-pass', got '%s'", cfg.LDAPAdminPass)
	}
}

func TestConfigMissingSecretsDir(t *testing.T) {
	os.Setenv("RUNTIME_SECRETS", "/nonexistent/path")
	defer os.Unsetenv("RUNTIME_SECRETS")

	// Should not error - missing files are tolerated
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OIDCClientSecret != "" {
		t.Fatalf("expected empty secret, got '%s'", cfg.OIDCClientSecret)
	}
}
