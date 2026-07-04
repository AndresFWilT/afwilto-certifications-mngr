package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

func newTestRouter(pingErr error) http.Handler {
	return NewRouter(RouterDeps{
		Pinger:       stubPinger{err: pingErr},
		AllowOrigins: []string{"http://localhost:3000"},
	})
}

func TestHealthzAlwaysOK(t *testing.T) {
	router := newTestRouter(errors.New("db down"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %s, want data.status ok", rec.Body.String())
	}
}

func TestReadyzOKWhenDBAnswers(t *testing.T) {
	router := newTestRouter(nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyzUnavailableWhenDBDown(t *testing.T) {
	router := newTestRouter(errors.New("connection refused"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"internal"`) {
		t.Errorf("body = %s, want error envelope", rec.Body.String())
	}
}

func TestCORSEchoesConfiguredOrigin(t *testing.T) {
	router := newTestRouter(nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Allow-Origin = %q, want the configured origin", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	router := newTestRouter(nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://evil.test")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty for unknown origin", got)
	}
}

func TestPreflightNoContent(t *testing.T) {
	router := newTestRouter(nil)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/certifications", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
