package unit

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// Sign with 12h TTL, single principal (no SSH roles)
	certBytes, err := sshCA.SignPublicKey(userPubBytes, []string{"testuser"}, 43200, 1)
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

func TestCAStampsRolePrincipals(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _ := ssh.NewPublicKey(userPriv.Public())
	userPubBytes := ssh.MarshalAuthorizedKey(userPub)

	// uid + two SSH login roles
	certBytes, err := sshCA.SignPublicKey(userPubBytes, []string{"alice", "ops", "dba"}, 43200, 2)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}
	cert := pubKey.(*ssh.Certificate)

	// KeyId stays the human uid (first principal) for audit.
	if cert.KeyId != "alice" {
		t.Fatalf("expected KeyId 'alice', got '%s'", cert.KeyId)
	}

	want := []string{"alice", "ops", "dba"}
	if len(cert.ValidPrincipals) != len(want) {
		t.Fatalf("expected %d principals, got %v", len(want), cert.ValidPrincipals)
	}
	for i, p := range want {
		if cert.ValidPrincipals[i] != p {
			t.Fatalf("principal %d: expected %q, got %q", i, p, cert.ValidPrincipals[i])
		}
	}
}

func TestCARejectsEmptyPrincipals(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _ := ssh.NewPublicKey(userPriv.Public())
	userPubBytes := ssh.MarshalAuthorizedKey(userPub)

	if _, err := sshCA.SignPublicKey(userPubBytes, nil, 43200, 3); err == nil {
		t.Fatal("expected error for empty principals")
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
	certBytes, err := sshCA.SignPublicKey(userPubBytes, []string{"admin"}, 0, 4)
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

	_, err = sshCA.SignPublicKey([]byte("not a valid key"), []string{"user"}, 3600, 5)
	if err == nil {
		t.Fatal("expected error for invalid public key")
	}
}

func TestCARotateChangesSigningKeyAndRetainsOld(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}
	before := sshCA.PublicKeyString()

	if err := sshCA.Rotate(); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	after := sshCA.PublicKeyString()

	if before == after {
		t.Fatal("signing key did not change after rotation")
	}

	// TrustedKeys must contain both the new (current) and the old (retired) key.
	trusted := string(sshCA.TrustedKeys())
	if !strings.Contains(trusted, strings.TrimSpace(after)) {
		t.Fatal("trusted set missing new key")
	}
	if !strings.Contains(trusted, strings.TrimSpace(before)) {
		t.Fatal("trusted set missing retired key")
	}

	// The retired private key file must be gone (only the current one remains).
	retiredDir := filepath.Join(dir, "ca", "retired")
	entries, err := os.ReadDir(retiredDir)
	if err != nil {
		t.Fatalf("reading retired dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 retired pub, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), ".pub") {
		t.Fatalf("retired entry should be public-only, got %s", entries[0].Name())
	}
}

func TestCARetiredKeysSurviveReload(t *testing.T) {
	dir := t.TempDir()

	ca1, err := ca.New(dir)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	oldPub := strings.TrimSpace(ca1.PublicKeyString())
	if err := ca1.Rotate(); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}

	// A fresh instance on the same dir must reload the retired pub.
	ca2, err := ca.New(dir)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if !strings.Contains(string(ca2.TrustedKeys()), oldPub) {
		t.Fatal("retired key not reloaded into trusted set after restart")
	}
}

func TestCACertSignedBeforeRotationVerifiesAgainstRetiredKey(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	_, userPriv, _ := ed25519.GenerateKey(rand.Reader)
	userPub, _ := ssh.NewPublicKey(userPriv.Public())
	userPubBytes := ssh.MarshalAuthorizedKey(userPub)

	certBytes, err := sshCA.SignPublicKey(userPubBytes, []string{"alice"}, 43200, 10)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	// Rotate after signing.
	if err := sshCA.Rotate(); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}

	pubKey, _, _, _, _ := ssh.ParseAuthorizedKey(certBytes)
	cert := pubKey.(*ssh.Certificate)

	// Build a checker that trusts the full published set (old + new).
	trusted := map[string]bool{}
	rest := sshCA.TrustedKeys()
	for len(rest) > 0 {
		var k ssh.PublicKey
		k, _, _, rest, err = ssh.ParseAuthorizedKey(rest)
		if err != nil {
			break
		}
		trusted[string(k.Marshal())] = true
	}

	checker := &ssh.CertChecker{
		IsUserAuthority: func(auth ssh.PublicKey) bool {
			return trusted[string(auth.Marshal())]
		},
	}
	if err := checker.CheckCert("alice", cert); err != nil {
		t.Fatalf("pre-rotation cert should still verify against retired key: %v", err)
	}
}

func TestCADeleteRetiredKey(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	// Rotate twice so there are two retired keys.
	firstPub := strings.TrimSpace(sshCA.PublicKeyString())
	if err := sshCA.Rotate(); err != nil {
		t.Fatalf("first rotate failed: %v", err)
	}
	secondPub := strings.TrimSpace(sshCA.PublicKeyString())
	if err := sshCA.Rotate(); err != nil {
		t.Fatalf("second rotate failed: %v", err)
	}

	retired := sshCA.RetiredKeys()
	if len(retired) != 2 {
		t.Fatalf("expected 2 retired keys, got %d", len(retired))
	}

	// Delete one retired key by its fingerprint.
	deletedFP := retired[0].Fingerprint
	keptFP := retired[1].Fingerprint
	if err := sshCA.DeleteRetiredKey(deletedFP); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	remaining := sshCA.RetiredKeys()
	if len(remaining) != 1 {
		t.Fatalf("expected 1 retired key after delete, got %d", len(remaining))
	}
	if remaining[0].Fingerprint != keptFP {
		t.Fatalf("expected kept fingerprint %s, got %s", keptFP, remaining[0].Fingerprint)
	}

	// The deleted key's fingerprint must be gone from the parsed trusted set,
	// while the still-retired second key remains present.
	trustedFPs := fingerprintsOf(t, sshCA.TrustedKeys())
	if trustedFPs[deletedFP] {
		t.Fatal("deleted key should not appear in trusted set")
	}
	if !trustedFPs[keptFP] {
		t.Fatal("kept retired key missing from trusted set")
	}

	// Its file must be removed from disk (one retired pub left).
	retiredDir := filepath.Join(dir, "ca", "retired")
	entries, _ := os.ReadDir(retiredDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 retired pub file on disk, got %d", len(entries))
	}
	_ = firstPub
	_ = secondPub
}

// fingerprintsOf parses an authorized_keys blob and returns the set of SHA256
// fingerprints it contains.
func fingerprintsOf(t *testing.T, blob []byte) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	rest := blob
	for len(rest) > 0 {
		k, _, _, r, err := ssh.ParseAuthorizedKey(rest)
		if err != nil {
			break
		}
		out[ssh.FingerprintSHA256(k)] = true
		rest = r
	}
	return out
}

func TestCADeleteRetiredKeyUnknownFingerprint(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}
	if err := sshCA.Rotate(); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}

	if err := sshCA.DeleteRetiredKey("SHA256:doesnotexist"); err == nil {
		t.Fatal("expected error deleting unknown fingerprint")
	}
	if len(sshCA.RetiredKeys()) != 1 {
		t.Fatal("retired set should be unchanged after failed delete")
	}
}

func TestCARetiredKeyHasRotatedAt(t *testing.T) {
	dir := t.TempDir()

	sshCA, err := ca.New(dir)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}
	before := time.Now().Add(-time.Second)
	if err := sshCA.Rotate(); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	retired := sshCA.RetiredKeys()
	if len(retired) != 1 {
		t.Fatalf("expected 1 retired key, got %d", len(retired))
	}
	if retired[0].RotatedAt.Before(before) {
		t.Fatalf("RotatedAt %v is before rotation time %v", retired[0].RotatedAt, before)
	}
}
