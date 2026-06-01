package unit

import (
	"testing"
	"time"

	"github.com/authbox/authbox/internal/backup"
	"github.com/authbox/authbox/internal/db"
)

func TestTruncateForRestoreClearsTargetTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	repo := db.NewRepository(database)

	// Seed data in all three target tables
	repo.CreateFIDO2Credential(&db.FIDO2Credential{UID: "user1", CredentialData: "cred1"})
	repo.CreateFIDO2Credential(&db.FIDO2Credential{UID: "user2", CredentialData: "cred2"})
	repo.CreateServiceAccount(&db.ServiceAccount{
		ClientID: "svc1", ClientSecretHash: "$2b$10$hash", Description: "test", Role: "viewer",
	})
	repo.CreateSSHCert(&db.SSHCert{
		Username: "user1", Serial: "aaa", Principal: "user1", ExpiresAt: time.Now().Add(time.Hour),
	})

	// Truncate
	if err := repo.TruncateForRestore(); err != nil {
		t.Fatalf("TruncateForRestore failed: %v", err)
	}

	// Verify all three tables are empty
	creds, _ := repo.GetAllFIDO2Credentials()
	if len(creds) != 0 {
		t.Fatalf("expected 0 fido2_credentials, got %d", len(creds))
	}

	accounts, _ := repo.ListServiceAccounts()
	if len(accounts) != 0 {
		t.Fatalf("expected 0 service_accounts, got %d", len(accounts))
	}

	certs, total, _ := repo.ListSSHCerts(0, 100)
	if len(certs) != 0 || total != 0 {
		t.Fatalf("expected 0 ssh_certs, got %d", total)
	}
}

func TestTruncateForRestoreDoesNotAffectEmployeeTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	repo := db.NewRepository(database)

	// employee_types is seeded by migration (4 defaults)
	before, _ := repo.ListEmployeeTypes()
	if len(before) == 0 {
		t.Fatal("expected seeded employee_types")
	}

	if err := repo.TruncateForRestore(); err != nil {
		t.Fatalf("TruncateForRestore failed: %v", err)
	}

	after, _ := repo.ListEmployeeTypes()
	if len(after) != len(before) {
		t.Fatalf("expected %d employee_types after truncate, got %d", len(before), len(after))
	}
}

func TestRestoreStateAfterTruncateRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DB test in short mode")
	}

	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()

	repo := db.NewRepository(database)

	// Insert initial data (simulates existing state)
	repo.CreateFIDO2Credential(&db.FIDO2Credential{UID: "alice", CredentialData: "key_alice"})
	repo.CreateServiceAccount(&db.ServiceAccount{
		ClientID: "ci-bot", ClientSecretHash: "$2b$10$hash", Description: "CI", Role: "operator",
	})

	// Build state to restore (same client_id - would conflict without truncate)
	state := &backup.ExportData{
		FIDO2: []db.FIDO2Credential{
			{UID: "alice", CredentialData: "key_alice"},
			{UID: "bob", CredentialData: "key_bob"},
		},
		ServiceAccounts: []db.ServiceAccount{
			{ClientID: "ci-bot", ClientSecretHash: "$2b$10$newhash", Description: "CI updated", Role: "admin"},
		},
		EmployeeTypes: []db.EmployeeType{
			{Value: "employee", Label: "Employee", Emoji: "👤", SortOrder: 1},
			{Value: "intern", Label: "Intern", Emoji: "🎓", SortOrder: 5},
		},
	}

	// RestoreState should succeed (truncates first, then inserts)
	if err := backup.RestoreState(repo, state); err != nil {
		t.Fatalf("RestoreState failed: %v", err)
	}

	// Verify FIDO2
	creds, _ := repo.GetAllFIDO2Credentials()
	if len(creds) != 2 {
		t.Fatalf("expected 2 fido2 credentials after restore, got %d", len(creds))
	}

	// Verify service accounts
	sa, err := repo.GetServiceAccountByClientID("ci-bot")
	if err != nil {
		t.Fatalf("expected ci-bot service account: %v", err)
	}
	if sa.Role != "admin" {
		t.Fatalf("expected role admin, got %s", sa.Role)
	}

	// Verify employee types include the new one
	types, _ := repo.ListEmployeeTypes()
	found := false
	for _, et := range types {
		if et.Value == "intern" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'intern' employee type after restore")
	}
}
