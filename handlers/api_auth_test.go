package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

func TestRequireAPISecret_missingHeader(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{APISecret: "longsecret"}
	h := RequireAPISecret(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireAPISecret_wrongSecret(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{APISecret: "longsecret"}
	h := RequireAPISecret(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-one")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireAPISecret_bearerOk(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{APISecret: "longsecret"}
	h := RequireAPISecret(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireAPISecret_xHeaderOk(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{APISecret: "longsecret"}
	h := RequireAPISecret(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Secret", "longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireAPISecret_prefersXHeaderWhenBothSet(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{APISecret: "longsecret"}
	h := RequireAPISecret(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Secret", "longsecret")
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("got %d want teapot", rec.Code)
	}
}

func TestRequireAPISecret_nilConfig(t *testing.T) {
	t.Parallel()
	h := RequireAPISecret(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireAPISecret_caseInsensitiveBearerPrefix(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{APISecret: "longsecret"}
	h := RequireAPISecret(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bEaReR longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("got %d", rec.Code)
	}
}
