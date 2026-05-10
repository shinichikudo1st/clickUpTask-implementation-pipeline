package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

func TestHealthHandler(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/health", nil)

	HealthHandler(nil)(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", recorder.Code, http.StatusOK)
	}

	var body models.SuccessResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Success {
		t.Fatal("success: want true")
	}
	data, ok := body.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data type: got %T", body.Data)
	}
	if data["status"] != "ok" {
		t.Fatalf("status field: got %v", data["status"])
	}
	if data["service"] != serviceName {
		t.Fatalf("service: got %v want %s", data["service"], serviceName)
	}
	if data["database"] != "not_configured" {
		t.Fatalf("database: got %v", data["database"])
	}
}
