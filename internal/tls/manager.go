package tls

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/authbox/authbox/internal/logging"
	"golang.org/x/crypto/acme"
)

const (
	renewBefore     = 30 * 24 * time.Hour // renew 30 days before expiry
	checkInterval   = 12 * time.Hour
	letsEncryptURL  = "https://acme-v02.api.letsencrypt.org/directory"
)

// Config holds TLS/ACME configuration.
type Config struct {
	CertPath        string
	KeyPath         string
	Domain          string
	ACMEEmail       string
	AWSAccessKeyID  string
	AWSSecretKey    string
	AWSHostedZoneID string
}

// Manager handles TLS certificate lifecycle.
type Manager struct {
	cfg    Config
	log    *logging.Logger
	mu     sync.RWMutex
	cert   *tls.Certificate
}

// NewManager creates a TLS manager.
func NewManager(cfg Config, log *logging.Logger) *Manager {
	return &Manager{cfg: cfg, log: log}
}

// GetCertificate returns the current TLS certificate for use in tls.Config.
func (m *Manager) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cert == nil {
		return nil, fmt.Errorf("no TLS certificate loaded")
	}
	return m.cert, nil
}

// EnsureCert loads an existing cert or obtains one via ACME.
// Blocks until a valid cert is available.
func (m *Manager) EnsureCert(ctx context.Context) error {
	// Try loading from disk first
	if err := m.loadFromDisk(); err == nil {
		m.log.Info("TLS certificate loaded from disk", "path", m.cfg.CertPath)
		return nil
	}

	// No cert on disk - obtain via ACME if configured
	if m.cfg.Domain != "" {
		m.log.Info("obtaining TLS certificate via ACME", "domain", m.cfg.Domain)
		return m.obtainCert(ctx)
	}

	// No ACME configured - generate self-signed for dev/testing
	m.log.Warn("no TLS cert found and TLS_DOMAIN not set - generating self-signed certificate")
	if err := GenerateSelfSigned(m.cfg.CertPath, m.cfg.KeyPath); err != nil {
		return fmt.Errorf("generating self-signed cert: %w", err)
	}
	return m.loadFromDisk()
}

// StartRenewalLoop checks cert expiry periodically and renews when needed.
func (m *Manager) StartRenewalLoop(ctx context.Context) {
	if m.cfg.Domain == "" {
		return // no ACME, no auto-renewal
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAndRenew(ctx)
		}
	}
}

func (m *Manager) checkAndRenew(ctx context.Context) {
	m.mu.RLock()
	cert := m.cert
	m.mu.RUnlock()

	if cert == nil {
		return
	}

	leaf := cert.Leaf
	if leaf == nil && len(cert.Certificate) > 0 {
		parsed, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return
		}
		leaf = parsed
	}

	if time.Until(leaf.NotAfter) > renewBefore {
		return // not yet time to renew
	}

	m.log.Info("TLS certificate expiring soon, renewing", "expires", leaf.NotAfter)
	if err := m.obtainCert(ctx); err != nil {
		m.log.Error("TLS renewal failed", "err", err)
	}
}

func (m *Manager) loadFromDisk() error {
	cert, err := tls.LoadX509KeyPair(m.cfg.CertPath, m.cfg.KeyPath)
	if err != nil {
		return err
	}
	// Parse leaf for expiry checking
	if len(cert.Certificate) > 0 {
		cert.Leaf, _ = x509.ParseCertificate(cert.Certificate[0])
	}
	m.mu.Lock()
	m.cert = &cert
	m.mu.Unlock()
	return nil
}

func (m *Manager) obtainCert(ctx context.Context) error {
	// Generate account key
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating account key: %w", err)
	}

	client := &acme.Client{
		Key:          accountKey,
		DirectoryURL: letsEncryptURL,
	}

	// Register account
	acct := &acme.Account{Contact: []string{"mailto:" + m.cfg.ACMEEmail}}
	if _, err := client.Register(ctx, acct, acme.AcceptTOS); err != nil {
		return fmt.Errorf("ACME register: %w", err)
	}

	// Create order
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(m.cfg.Domain))
	if err != nil {
		return fmt.Errorf("ACME authorize: %w", err)
	}

	// Process authorizations
	for _, authzURL := range order.AuthzURLs {
		authz, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return fmt.Errorf("get authz: %w", err)
		}

		var dns01 *acme.Challenge
		for _, ch := range authz.Challenges {
			if ch.Type == "dns-01" {
				dns01 = ch
				break
			}
		}
		if dns01 == nil {
			return fmt.Errorf("no dns-01 challenge offered")
		}

		// Get the TXT record value
		txtValue, err := client.DNS01ChallengeRecord(dns01.Token)
		if err != nil {
			return fmt.Errorf("dns01 record value: %w", err)
		}

		// Create DNS record
		recordName := "_acme-challenge." + m.cfg.Domain
		r53 := &route53Client{
			accessKeyID: m.cfg.AWSAccessKeyID,
			secretKey:   m.cfg.AWSSecretKey,
			hostedZone:  m.cfg.AWSHostedZoneID,
		}

		if err := r53.upsertTXTRecord(ctx, recordName, txtValue); err != nil {
			return fmt.Errorf("creating DNS record: %w", err)
		}

		// Wait for propagation
		m.log.Info("waiting for DNS propagation", "record", recordName)
		time.Sleep(30 * time.Second)

		// Accept challenge
		if _, err := client.Accept(ctx, dns01); err != nil {
			r53.deleteTXTRecord(ctx, recordName, txtValue)
			return fmt.Errorf("accepting challenge: %w", err)
		}

		// Wait for authorization
		if _, err := client.WaitAuthorization(ctx, authzURL); err != nil {
			r53.deleteTXTRecord(ctx, recordName, txtValue)
			return fmt.Errorf("waiting for authz: %w", err)
		}

		// Clean up DNS record
		r53.deleteTXTRecord(ctx, recordName, txtValue)
	}

	// Generate cert key
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating cert key: %w", err)
	}

	// Create CSR
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		DNSNames: []string{m.cfg.Domain},
	}, certKey)
	if err != nil {
		return fmt.Errorf("creating CSR: %w", err)
	}

	// Finalize order
	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return fmt.Errorf("finalizing order: %w", err)
	}

	// Save to disk
	if err := m.saveCert(der, certKey); err != nil {
		return fmt.Errorf("saving cert: %w", err)
	}

	// Reload
	return m.loadFromDisk()
}

func (m *Manager) saveCert(derChain [][]byte, key crypto.PrivateKey) error {
	if err := os.MkdirAll(filepath.Dir(m.cfg.CertPath), 0700); err != nil {
		return err
	}

	// Write cert chain
	certFile, err := os.OpenFile(m.cfg.CertPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer certFile.Close()
	for _, der := range derChain {
		pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	}

	// Write key
	keyBytes, err := x509.MarshalECPrivateKey(key.(*ecdsa.PrivateKey))
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfg.KeyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}), 0600)
}
