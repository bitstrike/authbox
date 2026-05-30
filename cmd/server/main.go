// main.go is the application entrypoint. Loads configuration, initializes all
// subsystems (SSH CA, SQLite, LDAP, OIDC, TLS), bootstraps the LDAP directory
// on first boot, wires up the chi router with API and frontend routes, starts
// background workers (replica sync, cert cleanup, TLS renewal), and serves
// HTTPS on port 8443. Supports a --obtain-cert mode for pre-start TLS
// provisioning from the container entrypoint.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/ca"
	"github.com/authbox/authbox/internal/config"
	"github.com/authbox/authbox/internal/db"
	appldap "github.com/authbox/authbox/internal/ldap"
	"github.com/authbox/authbox/internal/logging"
	appsync "github.com/authbox/authbox/internal/sync"
	apptls "github.com/authbox/authbox/internal/tls"
	"github.com/authbox/authbox/internal/web/api"
	"github.com/authbox/authbox/internal/web/frontend"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logging.New(cfg.LogLevel, cfg.LogDir)

	// --obtain-cert mode: get TLS cert and exit (used by entrypoint before slapd)
	if len(os.Args) > 1 && os.Args[1] == "--obtain-cert" {
		log.Info("obtaining TLS certificate", "domain", cfg.TLSDomain)
		tlsMgr := apptls.NewManager(apptls.Config{
			CertPath:        cfg.TLSCertPath,
			KeyPath:         cfg.TLSKeyPath,
			Domain:          cfg.TLSDomain,
			ACMEEmail:       cfg.TLSACMEEmail,
			AWSAccessKeyID:  cfg.AWSAccessKeyID,
			AWSSecretKey:    cfg.AWSSecretAccessKey,
			AWSHostedZoneID: cfg.AWSHostedZoneID,
		}, log)
		if err := tlsMgr.EnsureCert(context.Background()); err != nil {
			log.Error("failed to obtain TLS certificate", "err", err)
			os.Exit(1)
		}
		log.Info("TLS certificate ready", "path", cfg.TLSCertPath)
		os.Exit(0)
	}

	log.Info("authbox starting", "role", cfg.Role)

	// Initialize SSH CA
	sshCA, err := ca.New("/data")
	if err != nil {
		log.Error("failed to initialize SSH CA", "err", err)
		os.Exit(1)
	}
	log.Info("SSH CA initialized")

	// Initialize SQLite
	database, err := db.Open("/data")
	if err != nil {
		log.Error("failed to initialize database", "err", err)
		os.Exit(1)
	}
	defer database.Close()
	log.Info("database initialized")

	// Connect to LDAP
	ldapClient, err := appldap.NewClient(cfg.LDAPBaseDN, cfg.LDAPAdminPass)
	if err != nil {
		log.Error("failed to connect to LDAP", "err", err)
		os.Exit(1)
	}
	defer ldapClient.Close()

	// Bootstrap LDAP if primary
	if cfg.Role == "primary" {
		err = appldap.Bootstrap(ldapClient, appldap.BootstrapConfig{
			BaseDN:     cfg.LDAPBaseDN,
			AdminEmail: cfg.InitialAdmin,
			SchemaPath: "/app/ldif/schema.ldif",
		})
		if err != nil {
			log.Error("LDAP bootstrap failed", "err", err)
			os.Exit(1)
		}
		log.Info("LDAP bootstrap complete")
	}

	// Set up OIDC auth
	var authMiddleware func(http.Handler) http.Handler
	var oidcAuth *auth.OIDCAuth
	var roleLookup *appldap.RoleLookup

	if cfg.OIDCIssuerURL != "" && cfg.OIDCClientID != "" {
		oidcAuth, err = auth.NewOIDCAuth(context.Background(), auth.OIDCConfig{
			IssuerURL:    cfg.OIDCIssuerURL,
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  buildRedirectURL(cfg.TLSDomain),
		})
		if err != nil {
			log.Error("failed to initialize OIDC", "err", err)
			os.Exit(1)
		}
		roleLookup = appldap.NewRoleLookup(ldapClient)
		authMiddleware = auth.TokenMiddleware(oidcAuth.Verifier(), roleLookup, api.ValidateServiceToken)
		log.Info("OIDC authentication configured", "issuer", cfg.OIDCIssuerURL)
	} else {
		log.Warn("OIDC not configured - API authentication disabled")
		authMiddleware = func(next http.Handler) http.Handler {
			return next
		}
	}

	// Set up HTTP router
	repo := db.NewRepository(database)
	apiHandler := api.New(ldapClient, sshCA, repo, cfg.SSHCertTTL, cfg.InternalSecret)

	// Session store for web UI
	sessions := auth.NewSessionStore(30 * time.Minute)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(logging.Middleware(log))
	r.Use(middleware.Recoverer)

	// Replica: reject writes on API routes
	if cfg.Role == "replica" {
		r.Use(appsync.RejectWrites())
	}

	apiHandler.RegisterRoutesWithDeps(r, authMiddleware)

	// Frontend with OIDC login flow
	if oidcAuth != nil {
		authHandlers := frontend.NewAuthHandlers(oidcAuth, sessions, roleLookup)
		deps := &frontend.Deps{
			LDAP:     ldapClient,
			CA:       sshCA,
			Repo:     repo,
			Config:   cfg,
			Sessions: sessions,
			Roles:    roleLookup,
			Log:      log,
		}
		fe := frontend.NewFrontend(sessions, authHandlers, deps)
		fe.RegisterRoutes(r)
	} else {
		// Dev mode: no OIDC, auto-create admin session on first request
		log.Warn("OIDC not configured - running in dev mode with auto-login")
		deps := &frontend.Deps{
			LDAP:     ldapClient,
			CA:       sshCA,
			Repo:     repo,
			Config:   cfg,
			Sessions: sessions,
			Roles:    nil,
			Log:      log,
		}
		fe := frontend.NewFrontendDevMode(sessions, deps)
		fe.RegisterRoutes(r)
	}

	// Start replica sync loop if replica
	if cfg.Role == "replica" && cfg.PrimaryHost != "" {
		syncCtx, syncCancel := context.WithCancel(context.Background())
		defer syncCancel()
		rs := appsync.NewReplicaSync(cfg.PrimaryHost, cfg.InternalSecret, repo, log)
		go rs.Start(syncCtx)
	}

	// Background cleanup of expired cert records (daily, 90-day retention)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			deleted, err := repo.CleanExpiredCerts(90)
			if err != nil {
				log.Error("cert cleanup failed", "err", err)
			} else if deleted > 0 {
				log.Info("cleaned expired certs", "deleted", deleted)
			}
		}
	}()

	// TLS certificate management
	tlsMgr := apptls.NewManager(apptls.Config{
		CertPath:        cfg.TLSCertPath,
		KeyPath:         cfg.TLSKeyPath,
		Domain:          cfg.TLSDomain,
		ACMEEmail:       cfg.TLSACMEEmail,
		AWSAccessKeyID:  cfg.AWSAccessKeyID,
		AWSSecretKey:    cfg.AWSSecretAccessKey,
		AWSHostedZoneID: cfg.AWSHostedZoneID,
	}, log)

	if err := tlsMgr.EnsureCert(context.Background()); err != nil {
		log.Error("TLS certificate not available", "err", err)
		os.Exit(1)
	}

	// Start renewal loop
	go tlsMgr.StartRenewalLoop(context.Background())

	tlsCfg := &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: tlsMgr.GetCertificate,
	}

	srv := &http.Server{
		Addr:      ":8443",
		Handler:   r,
		TLSConfig: tlsCfg,
	}

	go func() {
		log.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "err", err)
	}
}

func buildRedirectURL(domain string) string {
	host := "localhost"
	if domain != "" {
		host = domain
	}
	return fmt.Sprintf("https://%s:8443/auth/callback", host)
}
