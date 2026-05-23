package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/authbox/authbox/internal/logging"
)

func TestLoggerWritesToFile(t *testing.T) {
	dir := t.TempDir()

	log := logging.New("info", dir)
	log.Info("test message", "key", "value")

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, "authbox-"+today+".log")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "[INFO]") {
		t.Fatalf("expected [INFO] in log, got: %s", content)
	}
	if !strings.Contains(content, "test message") {
		t.Fatalf("expected 'test message' in log, got: %s", content)
	}
	if !strings.Contains(content, "key=value") {
		t.Fatalf("expected 'key=value' in log, got: %s", content)
	}
}

func TestLoggerRespectsLevel(t *testing.T) {
	dir := t.TempDir()

	log := logging.New("warn", dir)
	log.Debug("should not appear")
	log.Info("should not appear")
	log.Warn("should appear")
	log.Error("should also appear")

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(dir, "authbox-"+today+".log")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "[DEBUG]") {
		t.Fatal("debug message should not appear at warn level")
	}
	if strings.Contains(content, "[INFO]") {
		t.Fatal("info message should not appear at warn level")
	}
	if !strings.Contains(content, "[WARN]") {
		t.Fatal("warn message should appear")
	}
	if !strings.Contains(content, "[ERROR]") {
		t.Fatal("error message should appear")
	}
}

func TestLoggerFallsBackToStdout(t *testing.T) {
	// Empty dir means no file logging - should not panic
	log := logging.New("info", "")
	log.Info("stdout message")
}

func TestLoggerCleanOldLogs(t *testing.T) {
	dir := t.TempDir()

	// Create a fake old log file
	oldFile := filepath.Join(dir, "authbox-2020-01-01.log")
	os.WriteFile(oldFile, []byte("old log"), 0640)
	// Set modification time to 100 days ago
	oldTime := time.Now().AddDate(0, 0, -100)
	os.Chtimes(oldFile, oldTime, oldTime)

	// Create logger - should trigger cleanup
	log := logging.New("info", dir)
	log.Info("trigger rotation")

	// Old file should be removed
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatal("expected old log file to be cleaned up")
	}
}
