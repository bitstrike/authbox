package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authbox/authbox/internal/flash"
)

func TestFlashSetWritesCookie(t *testing.T) {
	w := httptest.NewRecorder()
	flash.Set(w, flash.Success, "User created")

	resp := w.Result()
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected a cookie to be set")
	}

	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == "authbox_flash" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected authbox_flash cookie")
	}
	if found.MaxAge != 30 {
		t.Fatalf("expected MaxAge 30, got %d", found.MaxAge)
	}
	if !found.HttpOnly {
		t.Fatal("expected HttpOnly flag")
	}
	if !found.Secure {
		t.Fatal("expected Secure flag")
	}
}

func TestFlashGetReadsAndClears(t *testing.T) {
	// Set a flash
	w := httptest.NewRecorder()
	flash.Set(w, flash.Error, "something broke")
	resp := w.Result()

	// Build a request with that cookie
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range resp.Cookies() {
		req.AddCookie(c)
	}

	// Read the flash
	w2 := httptest.NewRecorder()
	msg := flash.Get(w2, req)
	if msg == nil {
		t.Fatal("expected a flash message")
	}
	if msg.Type != flash.Error {
		t.Fatalf("expected type error, got %s", msg.Type)
	}
	if msg.Text != "something broke" {
		t.Fatalf("expected 'something broke', got %q", msg.Text)
	}

	// Cookie should be cleared (MaxAge -1)
	clearCookies := w2.Result().Cookies()
	var cleared *http.Cookie
	for _, c := range clearCookies {
		if c.Name == "authbox_flash" {
			cleared = c
			break
		}
	}
	if cleared == nil {
		t.Fatal("expected clear cookie to be set")
	}
	if cleared.MaxAge != -1 {
		t.Fatalf("expected MaxAge -1 to clear cookie, got %d", cleared.MaxAge)
	}
}

func TestFlashGetReturnsNilWhenNoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	msg := flash.Get(w, req)
	if msg != nil {
		t.Fatalf("expected nil, got %+v", msg)
	}
}

func TestFlashGetHandlesMalformedCookie(t *testing.T) {
	// Cookie with no pipe separator
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "authbox_flash", Value: "garbage-no-pipe"})

	w := httptest.NewRecorder()
	msg := flash.Get(w, req)
	if msg != nil {
		t.Fatalf("expected nil for malformed cookie, got %+v", msg)
	}
}
