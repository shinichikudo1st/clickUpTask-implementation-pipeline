package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
	"github.com/go-chi/chi/v5"
)

func TestMethodNotAllowedHandler(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/v1/health", HealthHandler(nil))
	router.MethodNotAllowed(MethodNotAllowedHandler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/health", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body models.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code: %v", body.Error.Code)
	}
}
