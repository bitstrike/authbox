package unit

import (
	"testing"

	"github.com/authbox/authbox/internal/db"
)

func TestSetAndGetSetting(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	// Set a value
	if err := repo.SetSetting("backup_schedule_enabled", "true"); err != nil {
		t.Fatalf("SetSetting failed: %v", err)
	}

	// Get it back
	val, err := repo.GetSetting("backup_schedule_enabled")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if val != "true" {
		t.Fatalf("expected 'true', got '%s'", val)
	}
}

func TestSetSettingOverwrites(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	repo.SetSetting("key1", "first")
	repo.SetSetting("key1", "second")

	val, _ := repo.GetSetting("key1")
	if val != "second" {
		t.Fatalf("expected 'second', got '%s'", val)
	}
}

func TestGetSettingReturnsEmptyForMissing(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	val, err := repo.GetSetting("nonexistent_key")
	if err != nil {
		t.Fatalf("GetSetting returned error: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty string, got '%s'", val)
	}
}

func TestListSettings(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	repo.SetSetting("alpha", "1")
	repo.SetSetting("beta", "2")
	repo.SetSetting("gamma", "3")

	settings, err := repo.ListSettings()
	if err != nil {
		t.Fatalf("ListSettings failed: %v", err)
	}
	if len(settings) != 3 {
		t.Fatalf("expected 3 settings, got %d", len(settings))
	}
	// Should be ordered by key
	if settings[0].Key != "alpha" || settings[1].Key != "beta" || settings[2].Key != "gamma" {
		t.Fatalf("unexpected order: %v", settings)
	}
}
