package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authbox/authbox/internal/auth"
)

func TestSessionCreateAndGet(t *testing.T) {
	store := auth.NewSessionStore(30 * time.Minute)

	id := store.Create("user@example.com")
	if id == "" {
		t.Fatal("expected non-empty session ID")
	}

	sess := store.Get(id)
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.Email != "user@example.com" {
		t.Fatalf("expected email 'user@example.com', got '%s'", sess.Email)
	}
}

func TestSessionExpiry(t *testing.T) {
	store := auth.NewSessionStore(1 * time.Millisecond)

	id := store.Create("user@example.com")
	time.Sleep(5 * time.Millisecond)

	sess := store.Get(id)
	if sess != nil {
		t.Fatal("expected expired session to return nil")
	}
}

func TestSessionDelete(t *testing.T) {
	store := auth.NewSessionStore(30 * time.Minute)

	id := store.Create("user@example.com")
	store.Delete(id)

	sess := store.Get(id)
	if sess != nil {
		t.Fatal("expected deleted session to return nil")
	}
}

func TestSessionGetNonexistent(t *testing.T) {
	store := auth.NewSessionStore(30 * time.Minute)

	sess := store.Get("nonexistent")
	if sess != nil {
		t.Fatal("expected nil for nonexistent session")
	}
}

func TestSessionCookie(t *testing.T) {
	store := auth.NewSessionStore(30 * time.Minute)
	id := store.Create("user@example.com")

	w := httptest.NewRecorder()
	store.SetCookie(w, id)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie to be set")
	}

	cookie := cookies[0]
	if cookie.Name != auth.SessionCookieName {
		t.Fatalf("expected cookie name '%s', got '%s'", auth.SessionCookieName, cookie.Name)
	}
	if cookie.Value != id {
		t.Fatalf("expected cookie value '%s', got '%s'", id, cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}
	if !cookie.Secure {
		t.Fatal("expected Secure cookie")
	}
}

func TestSessionGetFromRequest(t *testing.T) {
	store := auth.NewSessionStore(30 * time.Minute)
	id := store.Create("user@example.com")

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: id})

	sess := store.GetFromRequest(r)
	if sess == nil {
		t.Fatal("expected session from request")
	}
	if sess.Email != "user@example.com" {
		t.Fatalf("expected email 'user@example.com', got '%s'", sess.Email)
	}
}

func TestSessionGetFromRequestNoCookie(t *testing.T) {
	store := auth.NewSessionStore(30 * time.Minute)

	r := httptest.NewRequest("GET", "/", nil)
	sess := store.GetFromRequest(r)
	if sess != nil {
		t.Fatal("expected nil when no cookie present")
	}
}

func TestSessionRefreshOnAccess(t *testing.T) {
	store := auth.NewSessionStore(100 * time.Millisecond)
	id := store.Create("user@example.com")

	// Access before expiry should refresh
	time.Sleep(50 * time.Millisecond)
	sess := store.Get(id)
	if sess == nil {
		t.Fatal("session should still be valid")
	}

	// After refresh, should survive another 50ms
	time.Sleep(50 * time.Millisecond)
	sess = store.Get(id)
	if sess == nil {
		t.Fatal("session should still be valid after refresh")
	}
}
