package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

const serviceName = "clickup-milestone-planner-service"

// HealthHandler returns 200 with service metadata. No external dependencies (Phase 0).
func HealthHandler(responseWriter http.ResponseWriter, _ *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(models.SuccessResponse{
		Success: true,
		Data: map[string]string{
			"status":    "ok",
			"service":   serviceName,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
		Error: nil,
	})
}
