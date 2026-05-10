package handlers

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/internal/clickupwebhook"
	"github.com/google/uuid"
)

const maxWebhookBodyBytes = 1 << 20

// MilestonePlanner runs the Phase 8 pipeline (ClickUp → generate → storage → email).
type MilestonePlanner interface {
	GenerateForTask(ctx context.Context, taskID string, force bool) error
}

// ClickUpWebhookHandler receives signed ClickUp webhooks, persists supported
// assignment-related events to clickup_events, and returns 200 with accepted metadata.
// Optional planner: when non-nil and the event row is newly inserted, runs GenerateForTask
// asynchronously (so the HTTP response is not blocked by LLM latency).
func ClickUpWebhookHandler(cfg *config.Config, store *db.Store, planner ...MilestonePlanner) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		if store == nil {
			WriteJSONError(responseWriter, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "database is required for webhooks")
			return
		}
		if strings.TrimSpace(cfg.ClickUpWebhookSecret) == "" {
			WriteJSONError(responseWriter, http.StatusUnauthorized, "UNAUTHORIZED", "CLICKUP_WEBHOOK_SECRET is not configured")
			return
		}

		bodyReader := http.MaxBytesReader(responseWriter, request.Body, maxWebhookBodyBytes)
		raw, err := io.ReadAll(bodyReader)
		if err != nil {
			// net/http MaxBytesReader returns a plain error string (not http.ErrBodyTooLarge on all Go versions).
			if strings.Contains(strings.ToLower(err.Error()), "body too large") {
				WriteJSONError(responseWriter, http.StatusRequestEntityTooLarge, "VALIDATION_ERROR", "request body too large")
				return
			}
			WriteJSONError(responseWriter, http.StatusBadRequest, "VALIDATION_ERROR", "unable to read body")
			return
		}
		if len(raw) == 0 {
			WriteJSONError(responseWriter, http.StatusBadRequest, "VALIDATION_ERROR", "empty body")
			return
		}

		signature := strings.TrimSpace(request.Header.Get("X-Signature"))
		if signature == "" {
			WriteJSONError(responseWriter, http.StatusUnauthorized, "UNAUTHORIZED", "missing X-Signature header")
			return
		}
		if !clickupwebhook.VerifyXSignature(raw, cfg.ClickUpWebhookSecret, signature) {
			WriteJSONError(responseWriter, http.StatusUnauthorized, "UNAUTHORIZED", "invalid webhook signature")
			return
		}

		payload, err := clickupwebhook.ParsePayload(raw)
		if err != nil {
			WriteJSONError(responseWriter, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}

		if payload.TaskID == "" {
			writeWebhookAccepted(responseWriter, false, "missing_task_id", uuid.Nil, false)
			return
		}

		related := clickupwebhook.IsAssignmentRelated(payload.Event)
		if payload.Event == "taskUpdated" && !clickupwebhook.TaskUpdatedAssigneeChange(raw) {
			related = false
		}
		if !related {
			writeWebhookAccepted(responseWriter, false, "unsupported_event", uuid.Nil, false)
			return
		}

		assigneeFilter := strings.TrimSpace(cfg.ClickUpAssigneeID)
		if assigneeFilter != "" {
			switch payload.Event {
			case "taskAssigneeUpdated", "taskUpdated":
				if !clickupwebhook.AssigneeAddMatchesUser(raw, assigneeFilter) {
					writeWebhookAccepted(responseWriter, false, "assignee_filter", uuid.Nil, false)
					return
				}
			}
		}

		dedupeKey, err := clickupwebhook.DedupeEventKey(raw)
		if err != nil {
			WriteJSONError(responseWriter, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}

		row := db.ClickUpEventRow{
			EventID:       sql.NullString{String: dedupeKey, Valid: true},
			ClickUpTaskID: sql.NullString{String: payload.TaskID, Valid: true},
			EventType:     payload.Event,
			PayloadJSON:   raw,
		}

		eventRowID, inserted, err := store.InsertClickUpEvent(request.Context(), row)
		if err != nil {
			WriteJSONError(responseWriter, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist event")
			return
		}

		if inserted && len(planner) > 0 && planner[0] != nil {
			p := planner[0]
			eid := eventRowID
			tid := payload.TaskID
			st := store
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
				defer cancel()
				err := p.GenerateForTask(ctx, tid, false)
				msg := sql.NullString{}
				if err != nil {
					msg = sql.NullString{String: truncateWebhookErr(err.Error()), Valid: true}
				}
				_ = st.MarkEventProcessed(ctx, eid, time.Now().UTC(), msg)
			}()
		}

		writeWebhookAccepted(responseWriter, true, "", eventRowID, !inserted)
	}
}

func truncateWebhookErr(s string) string {
	const max = 8000
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func writeWebhookAccepted(responseWriter http.ResponseWriter, accepted bool, reason string, eventRowID uuid.UUID, duplicate bool) {
	data := map[string]interface{}{
		"accepted": accepted,
	}
	if reason != "" {
		data["reason"] = reason
	}
	if accepted && eventRowID != uuid.Nil {
		data["event_row_id"] = eventRowID.String()
	}
	if accepted {
		data["duplicate"] = duplicate
	}
	WriteJSONSuccess(responseWriter, http.StatusOK, data)
}
