// helpers.go provides utility functions used across the frontend package:
// email-to-uid extraction and log file reading (tail and full). These are
// shared helpers that don't belong to a specific handler or component.
package frontend

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// emailToUID extracts the local part of an email address.
func emailToUID(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}

// readLogTail reads the last N lines from the most recent log file in dir.
func readLogTail(dir string, maxLines int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Find log files, sorted by name (date-based names sort chronologically)
	var logFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			logFiles = append(logFiles, e.Name())
		}
	}
	if len(logFiles) == 0 {
		return []string{"No log files found"}, nil
	}
	sort.Strings(logFiles)

	// Read the most recent file
	latest := filepath.Join(dir, logFiles[len(logFiles)-1])
	f, err := os.Open(latest)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Return last maxLines
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, scanner.Err()
}

// readLogFull reads all lines from the most recent log file in dir.
func readLogFull(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var logFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			logFiles = append(logFiles, e.Name())
		}
	}
	if len(logFiles) == 0 {
		return []string{"No log files found"}, nil
	}
	sort.Strings(logFiles)

	latest := filepath.Join(dir, logFiles[len(logFiles)-1])
	f, err := os.Open(latest)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
