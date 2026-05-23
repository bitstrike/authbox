package unit

import (
	"testing"
	"time"

	"github.com/authbox/authbox/internal/db"
)

func TestDBOpenAndMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode (modernc/sqlite may hang in some environments)")
	}

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	// Verify tables exist by querying them
	tables := []string{"schema_version", "service_accounts", "ssh_certs", "fido2_credentials", "sync_log"}
	for _, table := range tables {
		_, err := database.Conn().Query("SELECT 1 FROM " + table + " LIMIT 1")
		if err != nil {
			t.Fatalf("table %s does not exist: %v", table, err)
		}
	}
}

func TestDBIdempotentMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	// OpenMemory twice should not fail on IF NOT EXISTS
	db1, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("first open failed: %v", err)
	}
	db1.Close()

	db2, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	db2.Close()
}

func TestRepositoryFIDO2Credentials(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	repo := db.NewRepository(database)

	// Create credentials
	err = repo.CreateFIDO2Credential(&db.FIDO2Credential{
		UID:            "jsmith",
		CredentialData: "credential1_data_here",
	})
	if err != nil {
		t.Fatalf("failed to create credential: %v", err)
	}

	err = repo.CreateFIDO2Credential(&db.FIDO2Credential{
		UID:            "jsmith",
		CredentialData: "credential2_backup_key",
	})
	if err != nil {
		t.Fatalf("failed to create second credential: %v", err)
	}

	err = repo.CreateFIDO2Credential(&db.FIDO2Credential{
		UID:            "bdoe",
		CredentialData: "bdoe_credential",
	})
	if err != nil {
		t.Fatalf("failed to create credential for bdoe: %v", err)
	}

	// Get by user
	creds, err := repo.GetFIDO2Credentials("jsmith")
	if err != nil {
		t.Fatalf("failed to get credentials: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials for jsmith, got %d", len(creds))
	}

	// Get all
	all, err := repo.GetAllFIDO2Credentials()
	if err != nil {
		t.Fatalf("failed to get all credentials: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 total credentials, got %d", len(all))
	}

	// Delete by user
	err = repo.DeleteFIDO2Credentials("jsmith")
	if err != nil {
		t.Fatalf("failed to delete credentials: %v", err)
	}

	creds, _ = repo.GetFIDO2Credentials("jsmith")
	if len(creds) != 0 {
		t.Fatalf("expected 0 credentials after delete, got %d", len(creds))
	}

	// bdoe should still exist
	creds, _ = repo.GetFIDO2Credentials("bdoe")
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential for bdoe, got %d", len(creds))
	}
}

func TestRepositoryServiceAccount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	repo := db.NewRepository(database)

	err = repo.CreateServiceAccount(&db.ServiceAccount{
		ClientID:         "nightly-sync",
		ClientSecretHash: "$2b$10$hashhere",
		Description:      "Nightly user sync",
		Role:             "operator",
	})
	if err != nil {
		t.Fatalf("failed to create service account: %v", err)
	}

	// Duplicate client_id should fail
	err = repo.CreateServiceAccount(&db.ServiceAccount{
		ClientID:         "nightly-sync",
		ClientSecretHash: "$2b$10$otherhash",
		Description:      "Duplicate",
		Role:             "viewer",
	})
	if err == nil {
		t.Fatal("expected error for duplicate client_id")
	}
}

func TestRepositorySSHCert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	repo := db.NewRepository(database)

	err = repo.CreateSSHCert(&db.SSHCert{
		Username:  "jsmith",
		Serial:    "abc123",
		Principal: "jsmith",
		ExpiresAt: time.Now().Add(12 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create ssh cert record: %v", err)
	}
}
