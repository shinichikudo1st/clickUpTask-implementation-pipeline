package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

func TestAuth_MissingKeyUnauthorized(t *testing.T) {
	handler := Auth("secret-key-here")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body models.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "UNAUTHORIZED" {
		t.Fatalf("code: %v", body.Error.Code)
	}
}

func TestAuth_XAPIKeyAccepted(t *testing.T) {
	handler := Auth("secret-key-here")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-API-KEY", "secret-key-here")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status: %d", recorder.Code)
	}
}

func TestAuth_WrongKeyUnauthorized(t *testing.T) {
	handler := Auth("secret-key-here")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next must not run")
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-API-KEY", "wrong")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", recorder.Code)
	}
}

func TestAuth_ConstantTimeLengthMismatch(t *testing.T) {
	handler := Auth("short")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next must not run")
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-API-KEY", "much-longer-wrong-value")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", recorder.Code)
	}
}
