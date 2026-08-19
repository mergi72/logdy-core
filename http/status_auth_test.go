// Modified by VFS Platform contributors, 2026.
package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/logdyhq/logdy-core/models"
)

func statusTestConfig(t *testing.T) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logdy.config.json")
	if err := os.WriteFile(path, []byte(`{"columns":[{"title":"private"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return &Config{UiPass: "secret", ConfigFilePath: path, HttpPathPrefix: "/"}
}

func readInitMessage(t *testing.T, recorder *httptest.ResponseRecorder) models.InitMessage {
	t.Helper()
	var message models.InitMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &message); err != nil {
		t.Fatalf("invalid status response: %v", err)
	}
	return message
}

func TestStatusHidesConfigWithoutAuthentication(t *testing.T) {
	config := statusTestConfig(t)
	auth := newSessionAuth(config.UiPass, config.HttpPathPrefix)
	recorder := httptest.NewRecorder()
	handleStatus(config, auth)(recorder, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8801/api/status", nil))

	message := readInitMessage(t, recorder)
	if !message.AuthRequired {
		t.Fatal("status did not advertise required authentication")
	}
	if message.ConfigStr != "" {
		t.Fatal("status exposed configuration without authentication")
	}
	if message.AnalyticsEnabled {
		t.Fatal("status exposed operational settings without authentication")
	}
}

func TestStatusReturnsConfigWithAuthenticatedSession(t *testing.T) {
	config := statusTestConfig(t)
	auth := newSessionAuth(config.UiPass, config.HttpPathPrefix)
	auth.sessions["valid"] = time.Now().Add(time.Hour)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8801/api/status", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid"})
	recorder := httptest.NewRecorder()
	handleStatus(config, auth)(recorder, req)

	message := readInitMessage(t, recorder)
	if message.ConfigStr == "" {
		t.Fatal("authenticated status omitted configuration")
	}
}

func TestStatusReturnsConfigWhenAuthenticationIsDisabled(t *testing.T) {
	config := statusTestConfig(t)
	config.UiPass = ""
	auth := newSessionAuth("", config.HttpPathPrefix)
	recorder := httptest.NewRecorder()
	handleStatus(config, auth)(recorder, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8801/api/status", nil))

	message := readInitMessage(t, recorder)
	if message.ConfigStr == "" {
		t.Fatal("status omitted configuration when authentication is disabled")
	}
}
