package unit

import (
	"testing"
	"time"
)

// Test the TTL parsing logic (mirrors api.certTTLSeconds)
func TestCertTTLParsing(t *testing.T) {
	tests := []struct {
		name string
		input string
		want uint64
	}{
		{"12 hours", "12h", 43200},
		{"8 hours", "8h", 28800},
		{"1 hour", "1h", 3600},
		{"30 minutes", "30m", 1800},
		{"empty defaults to 12h", "", 43200},
		{"invalid defaults to 12h", "notaduration", 43200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTTL(tt.input)
			if got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func parseTTL(s string) uint64 {
	if s == "" {
		return 43200
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 43200
	}
	return uint64(d.Seconds())
}

// Test emailToUID extraction
func TestEmailToUID(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{"jsmith@example.com", "jsmith"},
		{"admin@company.org", "admin"},
		{"user.name@domain.co.uk", "user.name"},
		{"noatsign", "noatsign"},
		{"@leading", ""},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			got := emailToUID(tt.email)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func emailToUID(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}
