package http

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestSameHostOrigin(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "browser same host", host: "127.0.0.1:8801", origin: "http://127.0.0.1:8801", want: true},
		{name: "browser different host", host: "127.0.0.1:8801", origin: "https://example.com", want: false},
		{name: "non browser client", host: "127.0.0.1:8801", origin: "", want: true},
		{name: "malformed origin", host: "127.0.0.1:8801", origin: "://bad", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://"+tt.host+"/ws", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			if got := sameHostOrigin(req); got != tt.want {
				t.Fatalf("sameHostOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigSaveRejectsCrossOrigin(t *testing.T) {
	req := httptest.NewRequest("POST", "http://127.0.0.1:8801/api/config/save", bytes.NewBufferString(`{"layout":"{}"}`))
	req.Host = "127.0.0.1:8801"
	req.Header.Set("Origin", "https://example.com")
	recorder := httptest.NewRecorder()

	handleClientSettingsSave()(recorder, req)

	if recorder.Code != 403 {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}
}

func TestConfigSaveRejectsGet(t *testing.T) {
	req := httptest.NewRequest("GET", "http://127.0.0.1:8801/api/config/save", nil)
	recorder := httptest.NewRecorder()

	handleClientSettingsSave()(recorder, req)

	if recorder.Code != 405 {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}
