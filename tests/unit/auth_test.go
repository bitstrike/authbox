package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authbox/authbox/internal/auth"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid bearer", "Bearer abc123", "abc123"},
		{"case insensitive", "bearer xyz", "xyz"},
		{"empty", "", ""},
		{"no bearer prefix", "Basic abc123", ""},
		{"bearer only", "Bearer", ""},
		{"token with spaces", "Bearer token with spaces", "token with spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			got := auth.ExtractBearerToken(r)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name     string
		roles    []auth.Role
		required auth.Role
		want     bool
	}{
		{"admin has everything", []auth.Role{auth.RoleAdmin}, auth.RoleViewer, true},
		{"admin has operator", []auth.Role{auth.RoleAdmin}, auth.RoleOperator, true},
		{"viewer has viewer", []auth.Role{auth.RoleViewer}, auth.RoleViewer, true},
		{"viewer lacks operator", []auth.Role{auth.RoleViewer}, auth.RoleOperator, false},
		{"operator lacks admin", []auth.Role{auth.RoleOperator}, auth.RoleAdmin, false},
		{"self has self", []auth.Role{auth.RoleSelf}, auth.RoleSelf, true},
		{"self lacks viewer", []auth.Role{auth.RoleSelf}, auth.RoleViewer, false},
		{"empty roles", []auth.Role{}, auth.RoleViewer, false},
		{"multiple roles", []auth.Role{auth.RoleSelf, auth.RoleOperator}, auth.RoleOperator, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := auth.SetRoles(context.Background(), tt.roles)
			got := auth.HasRole(ctx, tt.required)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestRequireRoleMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allowed", func(t *testing.T) {
		ctx := auth.SetRoles(context.Background(), []auth.Role{auth.RoleAdmin})
		r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		auth.RequireRole(auth.RoleViewer)(handler).ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("forbidden", func(t *testing.T) {
		ctx := auth.SetRoles(context.Background(), []auth.Role{auth.RoleSelf})
		r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		auth.RequireRole(auth.RoleAdmin)(handler).ServeHTTP(w, r)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", w.Code)
		}
	})
}

func TestClaimsContext(t *testing.T) {
	claims := &auth.Claims{Email: "test@example.com", Sub: "sub123"}
	ctx := auth.SetClaims(context.Background(), claims)

	got := auth.GetClaims(ctx)
	if got == nil {
		t.Fatal("expected claims, got nil")
	}
	if got.Email != "test@example.com" {
		t.Fatalf("expected email 'test@example.com', got '%s'", got.Email)
	}
	if got.Sub != "sub123" {
		t.Fatalf("expected sub 'sub123', got '%s'", got.Sub)
	}
}

func TestGetClaimsEmpty(t *testing.T) {
	ctx := context.Background()
	got := auth.GetClaims(ctx)
	if got != nil {
		t.Fatal("expected nil claims from empty context")
	}
}
