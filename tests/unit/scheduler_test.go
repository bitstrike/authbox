package unit

import (
	"testing"
	"time"

	"github.com/authbox/authbox/internal/backup"
	"github.com/authbox/authbox/internal/db"
	"github.com/authbox/authbox/internal/logging"
)

func TestSchedulerReadSettingsDefaults(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)
	log := logging.New("error", t.TempDir())

	s := backup.NewScheduler(repo, "/usr/sbin/slapcat", t.TempDir(), log)
	defer s.Stop()

	enabled, timeStr, retention := s.ReadSettings()
	if enabled {
		t.Fatal("expected disabled by default")
	}
	if timeStr != "02:00" {
		t.Fatalf("expected default time '02:00', got '%s'", timeStr)
	}
	if retention != 30 {
		t.Fatalf("expected default retention 30, got %d", retention)
	}
}

func TestSchedulerReadSettingsFromDB(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)
	log := logging.New("error", t.TempDir())

	repo.SetSetting("backup_schedule_enabled", "true")
	repo.SetSetting("backup_schedule_time", "04:30")
	repo.SetSetting("backup_schedule_retention", "7")

	s := backup.NewScheduler(repo, "/usr/sbin/slapcat", t.TempDir(), log)
	defer s.Stop()

	enabled, timeStr, retention := s.ReadSettings()
	if !enabled {
		t.Fatal("expected enabled")
	}
	if timeStr != "04:30" {
		t.Fatalf("expected '04:30', got '%s'", timeStr)
	}
	if retention != 7 {
		t.Fatalf("expected 7, got %d", retention)
	}
}

func TestSchedulerDurationUntilFuture(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)
	log := logging.New("error", t.TempDir())

	s := backup.NewScheduler(repo, "/usr/sbin/slapcat", t.TempDir(), log)
	defer s.Stop()

	// Use a time far in the future relative to now to ensure it's "today"
	now := time.Now()
	futureHour := (now.Hour() + 2) % 24
	timeStr := time.Date(2000, 1, 1, futureHour, 30, 0, 0, time.UTC).Format("15:04")

	dur := s.DurationUntil(timeStr)
	if dur <= 0 {
		t.Fatalf("expected positive duration, got %v", dur)
	}
	if dur > 24*time.Hour {
		t.Fatalf("expected duration < 24h, got %v", dur)
	}
}

func TestSchedulerDurationUntilPastWrapsToNextDay(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)
	log := logging.New("error", t.TempDir())

	s := backup.NewScheduler(repo, "/usr/sbin/slapcat", t.TempDir(), log)
	defer s.Stop()

	// Use a time that's definitely in the past (1 hour ago)
	now := time.Now()
	pastHour := (now.Hour() + 23) % 24 // effectively -1 hour wrapped
	timeStr := time.Date(2000, 1, 1, pastHour, now.Minute(), 0, 0, time.UTC).Format("15:04")

	dur := s.DurationUntil(timeStr)
	if dur <= 0 {
		t.Fatalf("expected positive duration, got %v", dur)
	}
	// Should be roughly 23 hours (wraps to next day)
	if dur < 22*time.Hour || dur > 24*time.Hour {
		t.Fatalf("expected ~23h duration for past time, got %v", dur)
	}
}

func TestSchedulerStopIsIdempotent(t *testing.T) {
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repo := db.NewRepository(database)
	log := logging.New("error", t.TempDir())

	s := backup.NewScheduler(repo, "/usr/sbin/slapcat", t.TempDir(), log)
	s.Start()
	s.Stop()
	s.Stop() // should not panic
}
