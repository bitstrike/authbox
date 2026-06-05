// scheduler.go provides a background goroutine that runs daily slapcat backups
// at a configured time. Reads schedule settings from the app_settings table and
// arms a timer for the next run. Supports reconfiguration without restart.
package backup

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/authbox/authbox/internal/db"
	"github.com/authbox/authbox/internal/logging"
)

const (
	SettingBackupEnabled   = "backup_schedule_enabled"
	SettingBackupTime      = "backup_schedule_time"
	SettingBackupRetention = "backup_schedule_retention"

	defaultBackupTime      = "02:00"
	defaultBackupRetention = 30
)

// Scheduler runs daily backups on a timer.
type Scheduler struct {
	repo        *db.Repository
	slapcatPath string
	backupDir   string
	log         *logging.Logger

	mu      sync.Mutex
	timer   *time.Timer
	stopCh  chan struct{}
	stopped bool
}

// NewScheduler creates a backup scheduler.
func NewScheduler(repo *db.Repository, slapcatPath, backupDir string, log *logging.Logger) *Scheduler {
	return &Scheduler{
		repo:        repo,
		slapcatPath: slapcatPath,
		backupDir:   backupDir,
		log:         log,
		stopCh:      make(chan struct{}),
	}
}

// Start reads settings from DB and begins the schedule loop.
func (s *Scheduler) Start() {
	go s.loop()
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	close(s.stopCh)
	if s.timer != nil {
		s.timer.Stop()
	}
}

// Reconfigure re-reads settings and resets the timer.
func (s *Scheduler) Reconfigure() {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.mu.Unlock()
	// Send a zero-duration signal to re-enter the loop immediately
	// The loop checks settings on each iteration
	s.mu.Lock()
	if !s.stopped {
		s.timer = time.NewTimer(0)
	}
	s.mu.Unlock()
}

func (s *Scheduler) loop() {
	for {
		enabled, schedTime, retention := s.ReadSettings()

		var dur time.Duration
		if !enabled {
			// Not enabled: sleep 1 hour and re-check
			dur = 1 * time.Hour
		} else {
			dur = s.DurationUntil(schedTime)
		}

		s.mu.Lock()
		if s.stopped {
			s.mu.Unlock()
			return
		}
		s.timer = time.NewTimer(dur)
		s.mu.Unlock()

		select {
		case <-s.stopCh:
			return
		case <-s.timer.C:
			if !enabled {
				// Re-check after idle sleep
				continue
			}
			s.log.Info("running scheduled backup")
			if err := ScheduledBackup(s.slapcatPath, s.backupDir, retention); err != nil {
				s.log.Error("scheduled backup failed", "err", err)
			} else {
				s.log.Info("scheduled backup complete", "dir", s.backupDir)
			}
		}
	}
}

// ReadSettings reads backup schedule settings from the database.
func (s *Scheduler) ReadSettings() (enabled bool, timeStr string, retention int) {
	timeStr = defaultBackupTime
	retention = defaultBackupRetention

	val, _ := s.repo.GetSetting(SettingBackupEnabled)
	enabled = val == "true"

	if t, _ := s.repo.GetSetting(SettingBackupTime); t != "" {
		timeStr = t
	}

	if r, _ := s.repo.GetSetting(SettingBackupRetention); r != "" {
		if n, err := strconv.Atoi(r); err == nil && n > 0 {
			retention = n
		}
	}
	return
}

// DurationUntil calculates the duration until the next occurrence of the given time string (HH:MM).
func (s *Scheduler) DurationUntil(timeStr string) time.Duration {
	parts := strings.SplitN(timeStr, ":", 2)
	hour, minute := 2, 0
	if len(parts) == 2 {
		h, err := strconv.Atoi(parts[0])
		if err == nil {
			hour = h
		}
		m, err := strconv.Atoi(parts[1])
		if err == nil {
			minute = m
		}
	}

	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}
