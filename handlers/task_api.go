package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services/storage"
	"github.com/go-chi/chi/v5"
)

const maxGenerateBodyBytes = 16 << 10

// RegisterTaskAPI registers Phase 9 manual routes under /v1/tasks (caller mounts on main router).
func RegisterTaskAPI(router chi.Router, cfg *config.Config, store *db.Store, planner MilestonePlanner, blobs storage.BlobStore) {
	router.Route("/v1/tasks", func(r chi.Router) {
		r.Use(RequireAPISecret(cfg))
		r.Post("/{clickup_task_id}/generate", taskGenerateHandler(cfg, store, planner))
		r.Get("/{clickup_task_id}/plan", taskPlanHandler(cfg, store, blobs))
	})
}

type generateBody struct {
	Force *bool `json:"force"`
}

func taskGenerateHandler(cfg *config.Config, store *db.Store, planner MilestonePlanner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteJSONError(w, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "method not allowed")
			return
		}
		if store == nil {
			WriteJSONError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "database is required")
			return
		}
		if planner == nil {
			WriteJSONError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "milestone planner is not available")
			return
		}

		taskID := strings.TrimSpace(chi.URLParam(r, "clickup_task_id"))
		if taskID == "" {
			WriteJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "clickup_task_id is required")
			return
		}

		qv := r.URL.Query()
		force := false
		if _, ok := qv["force"]; ok {
			force = parseBoolishQuery(qv.Get("force"))
		} else if b, err := readGenerateJSON(w, r); err != nil {
			WriteJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body: "+err.Error())
			return
		} else if b != nil && b.Force != nil {
			force = *b.Force
		}

		tid := taskID
		p := planner
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			_ = p.GenerateForTask(ctx, tid, force)
		}()

		WriteJSONSuccess(w, http.StatusAccepted, map[string]interface{}{
			"clickup_task_id": tid,
			"force":           force,
			"status":          "started",
			"message":         "Generation started asynchronously; poll GET /v1/tasks/{clickup_task_id}/plan for the latest milestone_generations row.",
		})
	}
}

func readGenerateJSON(w http.ResponseWriter, r *http.Request) (*generateBody, error) {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if !strings.Contains(ct, "application/json") {
		return nil, nil
	}
	body := http.MaxBytesReader(w, r.Body, maxGenerateBodyBytes)
	raw, err := io.ReadAll(body)
	if err != nil {
		if isRequestEntityTooLarge(err) {
			return nil, errors.New("request body too large")
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, nil
	}
	var out generateBody
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func isRequestEntityTooLarge(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "body too large")
}

func parseBoolishQuery(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func taskPlanHandler(cfg *config.Config, store *db.Store, blobs storage.BlobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			WriteJSONError(w, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "method not allowed")
			return
		}
		if store == nil {
			WriteJSONError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "database is required")
			return
		}

		taskID := strings.TrimSpace(chi.URLParam(r, "clickup_task_id"))
		if taskID == "" {
			WriteJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "clickup_task_id is required")
			return
		}

		row, err := store.LatestGenerationByTaskID(r.Context(), taskID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				WriteJSONError(w, http.StatusNotFound, "NOT_FOUND", "no milestone generation found for this task")
				return
			}
			WriteJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load generation status")
			return
		}

		data := buildPlanPayload(cfg, blobs, row)
		WriteJSONSuccess(w, http.StatusOK, data)
	}
}

func buildPlanPayload(cfg *config.Config, blobs storage.BlobStore, row db.MilestoneGenerationRow) map[string]interface{} {
	gen := map[string]interface{}{
		"id":                 row.ID.String(),
		"status":             row.Status,
		"generation_version": row.GenerationVersion,
		"prompt_version":     row.PromptVersion,
		"model":              row.Model,
	}
	if row.FileName.Valid {
		gen["file_name"] = row.FileName.String
	}
	if row.StorageBucket.Valid {
		gen["storage_bucket"] = row.StorageBucket.String
	}
	if row.StoragePath.Valid {
		gen["storage_path"] = row.StoragePath.String
	}
	if row.MarkdownSHA256.Valid {
		gen["markdown_sha256"] = row.MarkdownSHA256.String
	}
	if row.ErrorMessage.Valid {
		gen["error_message"] = row.ErrorMessage.String
	}
	if row.StartedAt.Valid {
		gen["started_at"] = row.StartedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if row.CompletedAt.Valid {
		gen["completed_at"] = row.CompletedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if row.EmailSentAt.Valid {
		gen["email_sent_at"] = row.EmailSentAt.Time.UTC().Format(time.RFC3339Nano)
	}
	gen["created_at"] = row.CreatedAt.UTC().Format(time.RFC3339Nano)

	out := map[string]interface{}{
		"clickup_task_id": row.ClickUpTaskID,
		"generation":      gen,
	}

	if row.Status != "completed" {
		out["download_url"] = nil
		out["signed_url_available"] = false
		out["file_available"] = false
		if row.Status == "failed" {
			out["file_message"] = "generation failed; see generation.error_message"
		} else {
			out["file_message"] = "plan file is not available until status is completed"
		}
		return out
	}

	if !row.FileName.Valid || !row.StorageBucket.Valid || !row.StoragePath.Valid {
		out["download_url"] = nil
		out["signed_url_available"] = false
		out["file_available"] = false
		out["file_message"] = "completed generation is missing storage metadata"
		return out
	}

	if blobs == nil || cfg == nil {
		out["download_url"] = nil
		out["signed_url_available"] = false
		out["file_available"] = true
		out["file_message"] = "object storage is not configured for signed URLs on this service instance"
		return out
	}

	ttl := cfg.SignedURLTTL()
	if ttl <= 0 {
		out["download_url"] = nil
		out["signed_url_available"] = false
		out["file_available"] = true
		out["file_message"] = "signed URL TTL is not configured"
		return out
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, err := blobs.SignedDownloadURL(ctx, row.StorageBucket.String, row.StoragePath.String, ttl)
	if err != nil {
		if errors.Is(err, storage.ErrSignedURLUnsupported) {
			out["download_url"] = nil
			out["signed_url_available"] = false
			out["file_available"] = true
			out["file_message"] = "this storage backend does not support signed HTTP download URLs"
			return out
		}
		out["download_url"] = nil
		out["signed_url_available"] = false
		out["file_available"] = true
		out["file_message"] = "could not create a signed download URL"
		return out
	}

	out["download_url"] = u
	out["signed_url_available"] = true
	out["file_available"] = true
	out["signed_url_ttl_seconds"] = int(ttl.Seconds())
	return out
}
