package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionAuthDoesNotAcceptPasswordInURL(t *testing.T) {
	auth := newSessionAuth("secret", "/")
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8801/api/check-pass?password=secret", nil)
	recorder := httptest.NewRecorder()
	auth.check(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestSessionAuthLoginAndCookie(t *testing.T) {
	auth := newSessionAuth("secret", "/debugger/")
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8801/debugger/api/check-pass", bytes.NewBufferString(`{"password":"secret","remember":true}`))
	req.Host = "127.0.0.1:8801"
	req.Header.Set("Origin", "http://127.0.0.1:8801")
	recorder := httptest.NewRecorder()
	auth.check(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/debugger/" || cookie.MaxAge <= 0 {
		t.Fatalf("unexpected session cookie: %#v", cookie)
	}

	checkReq := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8801/debugger/api/check-pass", nil)
	checkReq.AddCookie(cookie)
	if !auth.authenticated(checkReq) {
		t.Fatal("issued session cookie was not accepted")
	}
}

func TestSessionAuthRejectsCrossOriginLogin(t *testing.T) {
	auth := newSessionAuth("secret", "/")
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8801/api/check-pass", bytes.NewBufferString(`{"password":"secret"}`))
	req.Host = "127.0.0.1:8801"
	req.Header.Set("Origin", "https://example.com")
	recorder := httptest.NewRecorder()
	auth.check(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
