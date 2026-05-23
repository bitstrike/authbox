package unit

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/authbox/authbox/internal/ca"
	"golang.org/x/crypto/ssh"
)

func TestCAGeneratesKeyOnFirstBoot(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	pubKey := sshCA.PublicKey()
	if len(pubKey) == 0 {
		t.Fatal("public key is empty")
	}

	// Verify key files were written
	keyPath := filepath.Join(dir, "ca", "ca_ed25519")
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("private key file not found: %v", err)
	}

	pubPath := keyPath + ".pub"
	if _, err := os.Stat(pubPath); err != nil {
		t.Fatalf("public key file not found: %v", err)
	}
}

func TestCALoadsExistingKey(t *testing.T) {
	dir := t.TempDir()

	// First init generates the key
	ca1, err := ca.New(dir)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	pub1 := ca1.PublicKey()

	// Second init should load the same key
	ca2, err := ca.New(dir)
	if err != nil {
		t.Fatalf("second init failed: %v", err)
	}
	pub2 := ca2.PublicKey()

	if string(pub1) != string(pub2) {
		t.Fatal("public keys differ after reload")
	}
}

func TestCASignsPublicKey(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	// Generate a user key to sign
	_, userPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate user key: %v", err)
	}
	userPub, err := ssh.NewPublicKey(userPriv.Public())
	if err != nil {
		t.Fatalf("failed to convert user public key: %v", err)
	}
	userPubBytes := ssh.MarshalAuthorizedKey(userPub)

	// Sign with 12h TTL
	certBytes, err := sshCA.SignPublicKey(userPubBytes, "testuser", 43200)
	if err != nil {
		t.Fatalf("failed to sign public key: %v", err)
	}

	if len(certBytes) == 0 {
		t.Fatal("signed certificate is empty")
	}

	// Parse the certificate
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		t.Fatalf("failed to parse signed cert: %v", err)
	}

	cert, ok := pubKey.(*ssh.Certificate)
	if !ok {
		t.Fatal("parsed key is not a certificate")
	}

	if cert.CertType != ssh.UserCert {
		t.Fatalf("expected UserCert, got %d", cert.CertType)
	}

	if cert.KeyId != "testuser" {
		t.Fatalf("expected KeyId 'testuser', got '%s'", cert.KeyId)
	}

	if len(cert.ValidPrincipals) != 1 || cert.ValidPrincipals[0] != "testuser" {
		t.Fatalf("unexpected principals: %v", cert.ValidPrincipals)
	}

	if cert.ValidBefore == ssh.CertTimeInfinity {
		t.Fatal("expected finite validity, got infinity")
	}
}

func TestCASignsWithInfiniteTTL(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _ := ssh.NewPublicKey(userPriv.Public())
	userPubBytes := ssh.MarshalAuthorizedKey(userPub)

	// TTL of 0 means no expiry
	certBytes, err := sshCA.SignPublicKey(userPubBytes, "admin", 0)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	pubKey, _, _, _, _ := ssh.ParseAuthorizedKey(certBytes)
	cert := pubKey.(*ssh.Certificate)

	if cert.ValidBefore != ssh.CertTimeInfinity {
		t.Fatalf("expected infinity, got %d", cert.ValidBefore)
	}
}

func TestCARejectsInvalidPublicKey(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	_, err = sshCA.SignPublicKey([]byte("not a valid key"), "user", 3600)
	if err == nil {
		t.Fatal("expected error for invalid public key")
	}
}
