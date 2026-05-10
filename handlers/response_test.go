package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

func TestWriteJSONError_EnvelopeShape(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteJSONError(recorder, http.StatusBadRequest, "VALIDATION_ERROR", "bad input")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body models.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Success {
		t.Fatal("success should be false")
	}
	if body.Data != nil {
		t.Fatalf("data should be null, got %#v", body.Data)
	}
	if body.Error.Code != "VALIDATION_ERROR" || body.Error.Message != "bad input" {
		t.Fatalf("error: %+v", body.Error)
	}
}

func TestWriteJSONSuccess_EnvelopeShape(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteJSONSuccess(recorder, http.StatusOK, map[string]string{"k": "v"})

	var body models.SuccessResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || body.Error != nil {
		t.Fatalf("body: %+v", body)
	}
}
