// Modified by VFS Platform contributors, 2026.
package http

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const sessionCookieName = "logdy_session"

type sessionAuth struct {
	password string
	path     string
	mu       sync.RWMutex
	sessions map[string]time.Time
}

func newSessionAuth(password, path string) *sessionAuth {
	return &sessionAuth{password: password, path: path, sessions: make(map[string]time.Time)}
}

func (a *sessionAuth) required() bool { return a.password != "" }

func (a *sessionAuth) authenticated(r *http.Request) bool {
	if !a.required() {
		return true
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	expires, ok := a.sessions[cookie.Value]
	if !ok {
		return false
	}
	if !time.Now().Before(expires) {
		delete(a.sessions, cookie.Value)
		return false
	}
	return true
}

func (a *sessionAuth) login(w http.ResponseWriter, r *http.Request) {
	if !a.required() {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost || !sameHostOrigin(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var request struct {
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Password != a.password {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	duration := 12 * time.Hour
	maxAge := 0
	if request.Remember {
		duration = 30 * 24 * time.Hour
		maxAge = int(duration.Seconds())
	}
	a.mu.Lock()
	a.sessions[token] = time.Now().Add(duration)
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     a.path,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	})
	w.WriteHeader(http.StatusOK)
}

func (a *sessionAuth) check(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.login(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !a.authenticated(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *sessionAuth) protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.authenticated(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
