package unit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/authbox/authbox/internal/backup"
	"github.com/authbox/authbox/internal/db"
)

func TestExportImportRoundTrip(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	// Seed data
	repo.CreateFIDO2Credential(&db.FIDO2Credential{
		UID:            "jsmith",
		CredentialData: "test-credential-data-abc123",
	})
	repo.CreateServiceAccount(&db.ServiceAccount{
		ClientID:         "sa-test-001",
		ClientSecretHash: "fakehash123",
		Description:      "test account",
		Role:             "viewer",
	})
	repo.CreateSSHCert(&db.SSHCert{
		Username:  "jsmith",
		Serial:    "ABCD1234",
		Principal: "jsmith",
		ExpiresAt: time.Now().Add(12 * time.Hour),
	})

	// Export (slapcat will fail since binary doesn't exist, but we can test
	// the archive structure by mocking - instead test ImportExport parsing)
	// Build a minimal archive manually using the package's exported types
	var buf bytes.Buffer
	err = backup.CreateExportFromState(&buf, repo)
	if err != nil {
		t.Fatal(err)
	}

	// Import
	imported, err := backup.ImportExport(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if imported.Meta.Version != 1 {
		t.Errorf("expected version 1, got %d", imported.Meta.Version)
	}
	if len(imported.State.FIDO2) != 1 {
		t.Errorf("expected 1 fido2 credential, got %d", len(imported.State.FIDO2))
	}
	if imported.State.FIDO2[0].UID != "jsmith" {
		t.Errorf("expected uid jsmith, got %s", imported.State.FIDO2[0].UID)
	}
	if imported.State.FIDO2[0].CredentialData != "test-credential-data-abc123" {
		t.Errorf("credential data mismatch")
	}
	if len(imported.State.ServiceAccounts) != 1 {
		t.Errorf("expected 1 service account, got %d", len(imported.State.ServiceAccounts))
	}
	if imported.State.ServiceAccounts[0].ClientID != "sa-test-001" {
		t.Errorf("expected client_id sa-test-001, got %s", imported.State.ServiceAccounts[0].ClientID)
	}
	if len(imported.State.SSHCerts) != 1 {
		t.Errorf("expected 1 ssh cert, got %d", len(imported.State.SSHCerts))
	}
}

func TestRestoreState(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	state := &backup.ExportData{
		FIDO2: []db.FIDO2Credential{
			{UID: "alice", CredentialData: "cred-alice"},
			{UID: "bob", CredentialData: "cred-bob"},
		},
		ServiceAccounts: []db.ServiceAccount{
			{ClientID: "sa-restore", ClientSecretHash: "hash", Description: "restored", Role: "admin"},
		},
	}

	err = backup.RestoreState(repo, state)
	if err != nil {
		t.Fatal(err)
	}

	// Verify FIDO2
	creds, err := repo.GetAllFIDO2Credentials()
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 2 {
		t.Errorf("expected 2 fido2 credentials, got %d", len(creds))
	}

	// Verify service accounts
	accounts, err := repo.ListServiceAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Errorf("expected 1 service account, got %d", len(accounts))
	}
	if accounts[0].ClientID != "sa-restore" {
		t.Errorf("expected client_id sa-restore, got %s", accounts[0].ClientID)
	}
}

func TestCleanOldBackups(t *testing.T) {
	dir := t.TempDir()

	// Create files with different ages
	oldFile := filepath.Join(dir, "backup-old.ldif")
	newFile := filepath.Join(dir, "backup-new.ldif")

	os.WriteFile(oldFile, []byte("old"), 0640)
	os.WriteFile(newFile, []byte("new"), 0640)

	// Set old file mtime to 100 days ago
	oldTime := time.Now().AddDate(0, 0, -100)
	os.Chtimes(oldFile, oldTime, oldTime)

	err := backup.CleanOldBackups(dir, 90)
	if err != nil {
		t.Fatal(err)
	}

	// Old file should be gone
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("expected old file to be deleted")
	}

	// New file should remain
	if _, err := os.Stat(newFile); err != nil {
		t.Error("expected new file to remain")
	}
}
