package unit

import (
	"bytes"
	"testing"

	"github.com/authbox/authbox/internal/backup"
	"github.com/authbox/authbox/internal/db"
)

func TestExportImportRoundTripWithAppSettings(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	// Seed app settings
	repo.SetSetting("backup_schedule_enabled", "true")
	repo.SetSetting("backup_schedule_time", "03:00")
	repo.SetSetting("backup_schedule_retention", "14")

	// Export
	var buf bytes.Buffer
	if err := backup.CreateExportFromState(&buf, repo); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Import into fresh DB
	database2, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database2.Close()
	repo2 := db.NewRepository(database2)

	imported, err := backup.ImportExport(&buf)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if err := backup.RestoreState(repo2, &imported.State); err != nil {
		t.Fatalf("RestoreState failed: %v", err)
	}

	// Verify settings restored
	val, _ := repo2.GetSetting("backup_schedule_enabled")
	if val != "true" {
		t.Fatalf("expected 'true', got '%s'", val)
	}
	val, _ = repo2.GetSetting("backup_schedule_time")
	if val != "03:00" {
		t.Fatalf("expected '03:00', got '%s'", val)
	}
	val, _ = repo2.GetSetting("backup_schedule_retention")
	if val != "14" {
		t.Fatalf("expected '14', got '%s'", val)
	}
}

func TestRestoreStateWithNoAppSettings(t *testing.T) {
	// Simulates importing an archive from before app_settings existed
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	// Pre-set a setting
	repo.SetSetting("existing_key", "existing_value")

	// Restore with nil/empty AppSettings
	state := &backup.ExportData{
		FIDO2:       []db.FIDO2Credential{},
		AppSettings: nil,
	}
	if err := backup.RestoreState(repo, state); err != nil {
		t.Fatalf("RestoreState failed: %v", err)
	}

	// Existing settings should remain (we don't truncate app_settings)
	val, _ := repo.GetSetting("existing_key")
	if val != "existing_value" {
		t.Fatalf("expected existing setting preserved, got '%s'", val)
	}
}
