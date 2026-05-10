package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

// WriteJSONSuccess writes an ApexSuite success envelope with the given HTTP status.
func WriteJSONSuccess(responseWriter http.ResponseWriter, statusCode int, data interface{}) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(models.SuccessResponse{
		Success: true,
		Data:    data,
		Error:   nil,
	})
}

// WriteJSONError writes an ApexSuite error envelope with the given HTTP status.
func WriteJSONError(responseWriter http.ResponseWriter, statusCode int, code, message string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(models.ErrorResponse{
		Success: false,
		Data:    nil,
		Error: models.ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}
