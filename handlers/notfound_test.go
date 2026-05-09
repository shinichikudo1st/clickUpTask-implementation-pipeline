package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

func TestNotFoundHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	NotFoundHandler(recorder, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body models.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Fatalf("code: %v", body.Error.Code)
	}
}
