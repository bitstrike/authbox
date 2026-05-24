package frontend

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DashboardStats holds all stat card values for the dashboard.
type DashboardStats struct {
	TotalUsers    int
	ActiveUsers   int
	DisabledUsers int
	GroupsTotal   int
	GroupsPosix   int
	GroupsRole    int
	CertsActive   int
	FIDO2Keys     int
	WarningsToday int
	ErrorsToday   int
}

// gatherDashboardStats queries LDAP, SQLite, and logs for dashboard data.
func (h *handlers) gatherDashboardStats() DashboardStats {
	var stats DashboardStats

	// Users from LDAP
	users, _, err := h.deps.LDAP.ListUsers(0, 10000)
	if err == nil {
		stats.TotalUsers = len(users)
		for _, u := range users {
			if u.Disabled {
				stats.DisabledUsers++
			} else {
				stats.ActiveUsers++
			}
		}
	}

	// Groups from LDAP
	groups, _, err := h.deps.LDAP.ListGroups(0, 10000)
	if err == nil {
		stats.GroupsTotal = len(groups)
		for _, g := range groups {
			if g.Type == "posixGroup" {
				stats.GroupsPosix++
			} else {
				stats.GroupsRole++
			}
		}
	}

	// Active certs (unexpired) from SQLite
	certs, _, err := h.deps.Repo.ListSSHCerts(0, 100000)
	if err == nil {
		now := time.Now()
		for _, c := range certs {
			if c.ExpiresAt.After(now) {
				stats.CertsActive++
			}
		}
	}

	// FIDO2 keys from SQLite
	fido2, err := h.deps.Repo.GetAllFIDO2Credentials()
	if err == nil {
		stats.FIDO2Keys = len(fido2)
	}

	// Log counts for today
	stats.WarningsToday, stats.ErrorsToday = countTodayLogLevels(h.deps.Config.LogDir)

	return stats
}

// countTodayLogLevels counts WARN and ERROR lines in today's log file.
func countTodayLogLevels(logDir string) (warnings, errors int) {
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(logDir, "authbox-"+today+".log")

	f, err := os.Open(logFile)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "[WARN]") {
			warnings++
		} else if strings.Contains(line, "[ERROR]") {
			errors++
		}
	}
	return warnings, errors
}
