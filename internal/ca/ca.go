// ca.go implements the SSH Certificate Authority. Generates or loads an
// ed25519 CA keypair from the data directory, signs user public keys with
// configurable TTL and principals, and exposes the CA public key and
// fingerprint for distribution to hosts.
package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// keyFileName is the current signing key's private key file (public is +".pub").
// retiredDirName holds public-only copies of keys retired by rotation, so hosts
// can keep trusting them during the dual-trust overlap window until certs expire.
const (
	keyFileName    = "ca_ed25519"
	retiredDirName = "retired"
)

// retiredKey is a public-only CA key kept trusted after a rotation, along with
// when it was rotated out and the file backing it on disk.
type retiredKey struct {
	pub       ssh.PublicKey
	rotatedAt time.Time
	file      string
}

// RetiredKey is the exported view of a retired key for the UI: its SHA256
// fingerprint and the time it was rotated out of the signing slot.
type RetiredKey struct {
	Fingerprint string
	RotatedAt   time.Time
}

type CA struct {
	mu          sync.RWMutex
	privateKey  ed25519.PrivateKey
	publicKey   ssh.PublicKey
	retiredKeys []retiredKey
	dataDir     string
}

func New(dataDir string) (*CA, error) {
	ca := &CA{dataDir: dataDir}
	if err := ca.loadOrGenerate(); err != nil {
		return nil, err
	}
	if err := ca.loadRetiredKeys(); err != nil {
		return nil, err
	}
	return ca, nil
}

// PublicKey returns the current signing key's public key. It intentionally does
// NOT include retired keys; use TrustedKeys for host distribution.
func (ca *CA) PublicKey() []byte {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ssh.MarshalAuthorizedKey(ca.publicKey)
}

func (ca *CA) PublicKeyString() string {
	return string(ca.PublicKey())
}

func (ca *CA) Fingerprint() string {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ssh.FingerprintSHA256(ca.publicKey)
}

// TrustedKeys returns the current signing key's public key plus every retired
// key's public key, newline-joined as authorized_keys lines. Hosts deploy this
// whole set to TrustedUserCAKeys so certificates signed before a rotation keep
// verifying until they expire.
func (ca *CA) TrustedKeys() []byte {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	out := ssh.MarshalAuthorizedKey(ca.publicKey)
	for _, k := range ca.retiredKeys {
		out = append(out, ssh.MarshalAuthorizedKey(k.pub)...)
	}
	return out
}

// RetiredKeys returns the retired (still-trusted) keys with their fingerprint
// and rotated-out time, for display in the UI.
func (ca *CA) RetiredKeys() []RetiredKey {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	out := make([]RetiredKey, 0, len(ca.retiredKeys))
	for _, k := range ca.retiredKeys {
		out = append(out, RetiredKey{
			Fingerprint: ssh.FingerprintSHA256(k.pub),
			RotatedAt:   k.rotatedAt,
		})
	}
	return out
}

// DeleteRetiredKey removes the retired key matching the given SHA256 fingerprint
// from disk and from the trusted set. Lookup is by fingerprint (never a
// client-supplied path) to avoid path traversal. Returns an error if no retired
// key matches. After deletion, re-running host enrollment drops the key from
// each host's TrustedUserCAKeys.
func (ca *CA) DeleteRetiredKey(fingerprint string) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	idx := -1
	for i, k := range ca.retiredKeys {
		if ssh.FingerprintSHA256(k.pub) == fingerprint {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("no retired key with fingerprint %s", fingerprint)
	}

	if err := os.Remove(ca.retiredKeys[idx].file); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing retired key file: %w", err)
	}
	ca.retiredKeys = append(ca.retiredKeys[:idx], ca.retiredKeys[idx+1:]...)
	return nil
}

// Rotate retires the current signing key (keeping its public key trusted) and
// generates a fresh ed25519 signing key. The old PRIVATE key is discarded - a
// compromise rotation must not leave it on disk. Hosts continue trusting the
// retired public key (via TrustedKeys) until certs signed by it expire; an
// operator prunes retired pubs after one SSH_CERT_TTL window.
func (ca *CA) Rotate() error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	keyPath := filepath.Join(ca.dataDir, "ca", keyFileName)
	pubPath := keyPath + ".pub"
	retiredDir := filepath.Join(ca.dataDir, "ca", retiredDirName)

	// Retain the current public key (public only) before overwriting.
	if err := os.MkdirAll(retiredDir, 0700); err != nil {
		return fmt.Errorf("creating retired dir: %w", err)
	}
	retiredPub := ca.publicKey
	rotatedAt := time.Now()
	// UnixNano (not Unix) so two rotations within the same second don't collide
	// on the same filename and clobber an earlier retired key.
	retiredPath := filepath.Join(retiredDir, fmt.Sprintf("%s.%d.pub", keyFileName, rotatedAt.UnixNano()))
	if err := os.WriteFile(retiredPath, ssh.MarshalAuthorizedKey(retiredPub), 0644); err != nil {
		return fmt.Errorf("writing retired public key: %w", err)
	}

	// Generate the new signing key and overwrite the current key files.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("converting public key: %w", err)
	}
	privBytes, err := ssh.MarshalPrivateKey(priv, "authbox CA key")
	if err != nil {
		return fmt.Errorf("marshaling private key: %w", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(privBytes), 0600); err != nil {
		return fmt.Errorf("writing private key: %w", err)
	}
	if err := os.WriteFile(pubPath, ssh.MarshalAuthorizedKey(sshPub), 0644); err != nil {
		return fmt.Errorf("writing public key: %w", err)
	}

	// Commit in-memory state only after all writes succeed.
	ca.privateKey = priv
	ca.publicKey = sshPub
	ca.retiredKeys = append(ca.retiredKeys, retiredKey{
		pub:       retiredPub,
		rotatedAt: rotatedAt,
		file:      retiredPath,
	})
	return nil
}

// loadRetiredKeys loads public-only retired keys from ca/retired/*.pub so the
// trusted set survives restarts. The rotated-out time is parsed from the
// <unixts> in the filename (ca_ed25519.<unixts>.pub), falling back to the file
// mtime if the name can't be parsed.
func (ca *CA) loadRetiredKeys() error {
	retiredDir := filepath.Join(ca.dataDir, "ca", retiredDirName)
	entries, err := os.ReadDir(retiredDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading retired dir: %w", err)
	}
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.retiredKeys = nil
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(retiredDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading retired key %s: %w", e.Name(), err)
		}
		pub, _, _, _, err := ssh.ParseAuthorizedKey(data)
		if err != nil {
			return fmt.Errorf("parsing retired key %s: %w", e.Name(), err)
		}
		ca.retiredKeys = append(ca.retiredKeys, retiredKey{
			pub:       pub,
			rotatedAt: rotatedAtFromName(e.Name(), path),
			file:      path,
		})
	}
	return nil
}

// rotatedAtFromName extracts the rotation time embedded in a retired key
// filename "ca_ed25519.<unixnano>.pub". Falls back to the file mtime, then to
// the zero time, if the timestamp can't be parsed.
func rotatedAtFromName(name, path string) time.Time {
	base := strings.TrimSuffix(name, ".pub")
	if i := strings.LastIndex(base, "."); i != -1 {
		if nanos, err := strconv.ParseInt(base[i+1:], 10, 64); err == nil {
			return time.Unix(0, nanos)
		}
	}
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// SignPublicKey signs a user public key. The first entry of principals is the
// caller's own uid, which is also recorded as the cert KeyId for audit; any
// additional entries are SSH login role principals (see internal/ldap/sshroles.go).
// All entries are stamped into ValidPrincipals so a single cert both self-logs
// and assumes role accounts.
func (ca *CA) SignPublicKey(pubKeyBytes []byte, principals []string, ttlSeconds uint64, serial uint64) ([]byte, error) {
	if len(principals) == 0 {
		return nil, fmt.Errorf("at least one principal is required")
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}

	ca.mu.RLock()
	signingKey := ca.privateKey
	ca.mu.RUnlock()

	signer, err := ssh.NewSignerFromKey(signingKey)
	if err != nil {
		return nil, fmt.Errorf("creating signer: %w", err)
	}

	cert := &ssh.Certificate{
		Key:             pubKey,
		CertType:        ssh.UserCert,
		KeyId:           principals[0],
		Serial:          serial,
		ValidPrincipals: principals,
		ValidAfter:      uint64(0),
		ValidBefore:     ssh.CertTimeInfinity,
		Permissions: ssh.Permissions{
			Extensions: map[string]string{
				"permit-pty":              "",
				"permit-agent-forwarding": "",
			},
		},
	}

	if ttlSeconds > 0 {
		now := unixNow()
		cert.ValidAfter = now - 60
		cert.ValidBefore = now + ttlSeconds
	}

	if err := cert.SignCert(rand.Reader, signer); err != nil {
		return nil, fmt.Errorf("signing certificate: %w", err)
	}

	return ssh.MarshalAuthorizedKey(cert), nil
}

func (ca *CA) loadOrGenerate() error {
	keyPath := filepath.Join(ca.dataDir, "ca", keyFileName)
	pubPath := keyPath + ".pub"

	if _, err := os.Stat(keyPath); err == nil {
		return ca.loadFromDisk(keyPath)
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return fmt.Errorf("creating CA directory: %w", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	ca.privateKey = priv
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("converting public key: %w", err)
	}
	ca.publicKey = sshPub

	privBytes, err := ssh.MarshalPrivateKey(priv, "authbox CA key")
	if err != nil {
		return fmt.Errorf("marshaling private key: %w", err)
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(privBytes), 0600); err != nil {
		return fmt.Errorf("writing private key: %w", err)
	}

	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	if err := os.WriteFile(pubPath, pubBytes, 0644); err != nil {
		return fmt.Errorf("writing public key: %w", err)
	}

	return nil
}

func (ca *CA) loadFromDisk(keyPath string) error {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("reading private key: %w", err)
	}

	rawKey, err := ssh.ParseRawPrivateKey(data)
	if err != nil {
		return fmt.Errorf("parsing private key: %w", err)
	}

	var priv ed25519.PrivateKey
	switch k := rawKey.(type) {
	case ed25519.PrivateKey:
		priv = k
	case *ed25519.PrivateKey:
		priv = *k
	default:
		return fmt.Errorf("expected ed25519 private key, got %T", rawKey)
	}

	ca.privateKey = priv
	pub := priv.Public().(ed25519.PublicKey)
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return fmt.Errorf("converting public key: %w", err)
	}
	ca.publicKey = sshPub
	return nil
}

func unixNow() uint64 {
	return uint64(time.Now().Unix())
}
