package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	SessionCookieName = "authbox_session"
	DefaultTimeout    = 30 * time.Minute
)

type Session struct {
	Email     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	timeout  time.Duration
}

func NewSessionStore(timeout time.Duration) *SessionStore {
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	s := &SessionStore{
		sessions: make(map[string]*Session),
		timeout:  timeout,
	}
	go s.cleanup()
	return s
}

func (s *SessionStore) Create(email string) string {
	id := generateSessionID()
	s.mu.Lock()
	s.sessions[id] = &Session{
		Email:     email,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.timeout),
	}
	s.mu.Unlock()
	return id
}

func (s *SessionStore) Get(id string) *Session {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	if time.Now().After(sess.ExpiresAt) {
		s.Delete(id)
		return nil
	}
	// Refresh expiry on access
	s.mu.Lock()
	sess.ExpiresAt = time.Now().Add(s.timeout)
	s.mu.Unlock()
	return sess
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *SessionStore) SetCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.timeout.Seconds()),
	})
}

func (s *SessionStore) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})
}

func (s *SessionStore) GetFromRequest(r *http.Request) *Session {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil
	}
	return s.Get(cookie.Value)
}

func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, sess := range s.sessions {
			if now.After(sess.ExpiresAt) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
