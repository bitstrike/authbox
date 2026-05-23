package ca

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

type CA struct {
	privateKey ed25519.PrivateKey
	publicKey  ssh.PublicKey
	dataDir    string
}

func New(dataDir string) (*CA, error) {
	ca := &CA{dataDir: dataDir}
	if err := ca.loadOrGenerate(); err != nil {
		return nil, err
	}
	return ca, nil
}

func (ca *CA) PublicKey() []byte {
	return ssh.MarshalAuthorizedKey(ca.publicKey)
}

func (ca *CA) SignPublicKey(pubKeyBytes []byte, principal string, ttlSeconds uint64) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(ca.privateKey)
	if err != nil {
		return nil, fmt.Errorf("creating signer: %w", err)
	}

	cert := &ssh.Certificate{
		Key:             pubKey,
		CertType:        ssh.UserCert,
		KeyId:           principal,
		ValidPrincipals: []string{principal},
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
	keyPath := filepath.Join(ca.dataDir, "ca", "ca_ed25519")
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
