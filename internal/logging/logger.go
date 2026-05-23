package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

type Logger struct {
	level      Level
	dir        string
	mu         sync.Mutex
	file       *os.File
	currentDay string
}

func New(level string, dir string) *Logger {
	l := &Logger{
		level: parseLevel(level),
		dir:   dir,
	}
	l.rotate()
	return l
}

func (l *Logger) Debug(msg string, args ...any) {
	l.log(LevelDebug, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.log(LevelInfo, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.log(LevelWarn, msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.log(LevelError, msg, args...)
}

func (l *Logger) log(level Level, msg string, args ...any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if today != l.currentDay {
		l.rotate()
	}

	ts := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	entry := fmt.Sprintf("%s [%s] %s", ts, levelString(level), msg)

	for i := 0; i < len(args)-1; i += 2 {
		entry += fmt.Sprintf(" %v=%v", args[i], args[i+1])
	}
	entry += "\n"

	w := l.writer()
	fmt.Fprint(w, entry)
}

func (l *Logger) writer() io.Writer {
	if l.file != nil {
		return l.file
	}
	return os.Stdout
}

func (l *Logger) rotate() {
	if l.dir == "" {
		return
	}

	if err := os.MkdirAll(l.dir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log dir: %v\n", err)
		return
	}

	today := time.Now().Format("2006-01-02")
	path := filepath.Join(l.dir, fmt.Sprintf("authbox-%s.log", today))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		return
	}

	if l.file != nil {
		l.file.Close()
	}
	l.file = f
	l.currentDay = today

	l.cleanOldLogs(90)
}

func (l *Logger) cleanOldLogs(retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(l.dir, e.Name()))
		}
	}
}

func parseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func levelString(l Level) string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}
