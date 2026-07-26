package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestPublicReadsAndProtectedWrites(t *testing.T) {
	manager := testManager(t)
	handler := testHandler(manager)

	read := httptest.NewRequest(http.MethodGet, "/api/players", nil)
	readRecorder := httptest.NewRecorder()
	handler.ServeHTTP(readRecorder, read)
	if readRecorder.Code != http.StatusOK {
		t.Fatalf("public read status = %d, want 200", readRecorder.Code)
	}

	write := httptest.NewRequest(http.MethodPost, "/api/players", bytes.NewBufferString(`{}`))
	writeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(writeRecorder, write)
	if writeRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous write status = %d, want 401", writeRecorder.Code)
	}
}

func TestLoginCreatesSessionThatAuthorizesWrites(t *testing.T) {
	manager := testManager(t)
	handler := testHandler(manager)

	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"correct horse battery staple"}`))
	login.Host = "golf.example"
	login.Header.Set("Origin", "https://golf.example")
	loginRecorder := httptest.NewRecorder()
	handler.ServeHTTP(loginRecorder, login)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	cookies := loginRecorder.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("login cookies = %#v, want one hardened session cookie", cookies)
	}

	write := httptest.NewRequest(http.MethodPost, "/api/players", bytes.NewBufferString(`{}`))
	write.Host = "golf.example"
	write.Header.Set("Origin", "https://golf.example")
	write.AddCookie(cookies[0])
	writeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(writeRecorder, write)
	if writeRecorder.Code != http.StatusNoContent {
		t.Fatalf("authenticated write status = %d, want 204; body %s", writeRecorder.Code, writeRecorder.Body.String())
	}
}

func TestInvalidLoginAndCrossOriginWriteAreRejected(t *testing.T) {
	manager := testManager(t)
	handler := testHandler(manager)

	badLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"wrong"}`))
	badRecorder := httptest.NewRecorder()
	handler.ServeHTTP(badRecorder, badLogin)
	if badRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login status = %d, want 401", badRecorder.Code)
	}

	cookie := loginCookie(t, handler)
	write := httptest.NewRequest(http.MethodDelete, "/api/rounds/1", nil)
	write.Host = "golf.example"
	write.Header.Set("Origin", "https://attacker.example")
	write.AddCookie(cookie)
	writeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(writeRecorder, write)
	if writeRecorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin write status = %d, want 403", writeRecorder.Code)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	manager := testManager(t)
	handler := testHandler(manager)
	cookie := loginCookie(t, handler)

	logout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logout.AddCookie(cookie)
	logoutRecorder := httptest.NewRecorder()
	handler.ServeHTTP(logoutRecorder, logout)
	if logoutRecorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", logoutRecorder.Code)
	}

	write := httptest.NewRequest(http.MethodPut, "/api/players/1", bytes.NewBufferString(`{}`))
	write.AddCookie(cookie)
	writeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(writeRecorder, write)
	if writeRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("write after logout status = %d, want 401", writeRecorder.Code)
	}
}

func TestUnconfiguredAuthenticationLeavesApplicationReadOnly(t *testing.T) {
	manager, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}
	handler := testHandler(manager)
	write := httptest.NewRequest(http.MethodPost, "/api/rounds", bytes.NewBufferString(`{}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, write)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured write status = %d, want 503", recorder.Code)
	}
}

func TestRepeatedLoginFailuresAreRateLimited(t *testing.T) {
	manager := testManager(t)
	handler := testHandler(manager)
	for attempt := 0; attempt < maxLoginAttempts; attempt++ {
		login := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"wrong"}`))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, login)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("failed login %d status = %d, want 401", attempt+1, recorder.Code)
		}
	}
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"wrong"}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, login)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited login status = %d, want 429", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response is missing Retry-After")
	}
}

func testManager(t *testing.T) *Manager {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(Config{
		PasswordHash:   string(hash),
		SecureCookie:   true,
		SessionTimeout: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func testHandler(manager *Manager) http.Handler {
	mux := http.NewServeMux()
	manager.Register(mux)
	mux.HandleFunc("GET /api/players", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /api/players", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /api/players/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/rounds/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return manager.ProtectWrites(mux)
}

func loginCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"correct horse battery staple"}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, login)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", recorder.Code)
	}
	return recorder.Result().Cookies()[0]
}
