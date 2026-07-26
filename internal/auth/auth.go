package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	cookieName       = "golf_admin_session"
	maxLoginAttempts = 10
	loginWindow      = 5 * time.Minute
)

type Config struct {
	Password       string
	PasswordHash   string
	SecureCookie   bool
	SessionTimeout time.Duration
}

type Manager struct {
	passwordHash []byte
	secureCookie bool
	sessionTTL   time.Duration
	now          func() time.Time
	random       io.Reader

	mu       sync.Mutex
	sessions map[string]time.Time
	attempts map[string]loginAttempts
}

type loginAttempts struct {
	count int
	reset time.Time
}

func New(config Config) (*Manager, error) {
	hash := []byte(config.PasswordHash)
	if len(hash) == 0 && config.Password != "" {
		generated, err := bcrypt.GenerateFromPassword([]byte(config.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hash = generated
	}
	if len(hash) > 0 {
		if _, err := bcrypt.Cost(hash); err != nil {
			return nil, errors.New("GOLF_ADMIN_PASSWORD_HASH is not a valid bcrypt hash")
		}
	}
	sessionTTL := config.SessionTimeout
	if sessionTTL <= 0 {
		sessionTTL = 12 * time.Hour
	}
	return &Manager{
		passwordHash: hash,
		secureCookie: config.SecureCookie,
		sessionTTL:   sessionTTL,
		now:          time.Now,
		random:       rand.Reader,
		sessions:     make(map[string]time.Time),
		attempts:     make(map[string]loginAttempts),
	}, nil
}

func (m *Manager) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/session", m.session)
	mux.HandleFunc("POST /api/auth/login", m.login)
	mux.HandleFunc("POST /api/auth/logout", m.logout)
}

func (m *Manager) ProtectWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isRead(r.Method) || r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/logout" {
			next.ServeHTTP(w, r)
			return
		}
		if len(m.passwordHash) == 0 {
			writeError(w, http.StatusServiceUnavailable, "admin authentication is not configured")
			return
		}
		if !m.authenticated(r) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Manager) session(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	writeJSON(w, http.StatusOK, map[string]bool{
		"authenticated": m.authenticated(r),
		"enabled":       len(m.passwordHash) > 0,
	})
}

func (m *Manager) login(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	if len(m.passwordHash) == 0 {
		writeError(w, http.StatusServiceUnavailable, "admin authentication is not configured")
		return
	}
	key := clientKey(r)
	if retryAfter := m.retryAfter(key); retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "too many login attempts; try again later")
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if err := bcrypt.CompareHashAndPassword(m.passwordHash, []byte(input.Password)); err != nil {
		m.recordFailure(key)
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}
	m.clearFailures(key)
	token, err := m.newSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	expires := m.now().Add(m.sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(m.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   m.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (m *Manager) logout(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	if cookie, err := r.Cookie(cookieName); err == nil {
		m.mu.Lock()
		delete(m.sessions, cookie.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (m *Manager) newSession() (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(m.random, raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneSessions(now)
	m.sessions[token] = now.Add(m.sessionTTL)
	return token, nil
}

func (m *Manager) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	expires, found := m.sessions[cookie.Value]
	if !found || !expires.After(now) {
		delete(m.sessions, cookie.Value)
		return false
	}
	return true
}

func (m *Manager) pruneSessions(now time.Time) {
	for token, expires := range m.sessions {
		if !expires.After(now) {
			delete(m.sessions, token)
		}
	}
}

func (m *Manager) retryAfter(key string) time.Duration {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	attempts := m.attempts[key]
	if !attempts.reset.After(now) {
		delete(m.attempts, key)
		return 0
	}
	if attempts.count < maxLoginAttempts {
		return 0
	}
	return attempts.reset.Sub(now)
}

func (m *Manager) recordFailure(key string) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	attempts := m.attempts[key]
	if !attempts.reset.After(now) {
		attempts = loginAttempts{reset: now.Add(loginWindow)}
	}
	attempts.count++
	m.attempts[key] = attempts
}

func (m *Manager) clearFailures(key string) {
	m.mu.Lock()
	delete(m.attempts, key)
	m.mu.Unlock()
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Host == r.Host
}

func isRead(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
