// flash.go provides server-side flash messages that survive a single redirect.
// Messages are stored in a short-lived cookie and cleared on read.
package flash

import (
	"net/http"
	"net/url"
	"strings"
)

const cookieName = "authbox_flash"

// Type represents the severity of a flash message.
type Type string

const (
	Success Type = "success"
	Error   Type = "error"
	Warning Type = "warning"
)

// Message holds a typed flash notification.
type Message struct {
	Type Type
	Text string
}

// Set writes a flash message cookie that will be read on the next request.
func Set(w http.ResponseWriter, t Type, text string) {
	val := url.QueryEscape(string(t) + "|" + text)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30,
	})
}

// Get reads and clears the flash message from the request. Returns nil if none.
func Get(w http.ResponseWriter, r *http.Request) *Message {
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	// Clear the cookie
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
	raw, err := url.QueryUnescape(c.Value)
	if err != nil {
		return nil
	}
	parts := strings.SplitN(raw, "|", 2)
	if len(parts) != 2 {
		return nil
	}
	return &Message{Type: Type(parts[0]), Text: parts[1]}
}
