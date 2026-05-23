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

	"github.com/authbox/authbox/internal/ca"
	"github.com/authbox/authbox/internal/config"
	"github.com/authbox/authbox/internal/db"
	"github.com/authbox/authbox/internal/ldap"
	"github.com/authbox/authbox/internal/logging"
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
	log.Info("authbox starting", "role", cfg.Role)

	// Initialize SSH CA (loads or generates keypair)
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

	// Connect to LDAP and bootstrap if needed
	ldapClient, err := ldap.NewClient(cfg.LDAPBaseDN, cfg.LDAPAdminPass)
	if err != nil {
		log.Error("failed to connect to LDAP", "err", err)
		os.Exit(1)
	}
	defer ldapClient.Close()

	if cfg.Role == "primary" {
		err = ldap.Bootstrap(ldapClient, ldap.BootstrapConfig{
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

	// Set up HTTP router
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(logging.Middleware(log))
	r.Use(middleware.Recoverer)

	api.RegisterRoutes(r)
	frontend.RegisterRoutes(r)

	_ = sshCA // used in Phase 4 when wiring SSH endpoints

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	srv := &http.Server{
		Addr:      ":8443",
		Handler:   r,
		TLSConfig: tlsCfg,
	}

	go func() {
		log.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath); err != nil && err != http.ErrServerClosed {
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
