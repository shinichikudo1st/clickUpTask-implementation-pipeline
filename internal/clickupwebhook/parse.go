package clickupwebhook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const maxDedupeKeyLen = 512

// Payload is the minimal ClickUp webhook fields this service needs.
type Payload struct {
	Event     string          `json:"event"`
	WebhookID string          `json:"webhook_id"`
	TaskID    string          `json:"task_id"`
	TeamID    string          `json:"team_id"`
	Raw       json.RawMessage `json:"-"`
}

type historyItem struct {
	ID    string          `json:"id"`
	Field string          `json:"field"`
	After json.RawMessage `json:"after"`
}

type payloadWithHistory struct {
	Event        string        `json:"event"`
	WebhookID    string        `json:"webhook_id"`
	TaskID       string        `json:"task_id"`
	TeamID       string        `json:"team_id"`
	HistoryItems []historyItem `json:"history_items"`
}

// ParsePayload validates JSON and extracts core fields.
func ParsePayload(raw []byte) (Payload, error) {
	var inner payloadWithHistory
	if err := json.Unmarshal(raw, &inner); err != nil {
		return Payload{}, fmt.Errorf("invalid json: %w", err)
	}
	if strings.TrimSpace(inner.Event) == "" {
		return Payload{}, fmt.Errorf("missing event")
	}
	if strings.TrimSpace(inner.WebhookID) == "" {
		return Payload{}, fmt.Errorf("missing webhook_id")
	}
	return Payload{
		Event:     strings.TrimSpace(inner.Event),
		WebhookID: strings.TrimSpace(inner.WebhookID),
		TaskID:    strings.TrimSpace(inner.TaskID),
		TeamID:    strings.TrimSpace(inner.TeamID),
		Raw:       json.RawMessage(raw),
	}, nil
}

// DedupeEventKey returns a stable idempotency key for clickup_events.event_id.
// Prefer webhook_id:history_item_id when available (ClickUp recommendation).
func DedupeEventKey(raw []byte) (string, error) {
	var inner payloadWithHistory
	if err := json.Unmarshal(raw, &inner); err != nil {
		return "", err
	}
	if inner.WebhookID == "" {
		return "", fmt.Errorf("missing webhook_id")
	}
	for _, item := range inner.HistoryItems {
		id := strings.TrimSpace(item.ID)
		if id != "" {
			key := inner.WebhookID + ":" + id
			if len(key) > maxDedupeKeyLen {
				return "", fmt.Errorf("dedupe key too long")
			}
			return key, nil
		}
	}
	sum := sha256.Sum256(raw)
	return "body:" + hex.EncodeToString(sum[:]), nil
}

// IsAssignmentRelated reports whether this event type can trigger the pipeline
// (subject to optional assignee filter and taskCreated skip in the HTTP handler).
func IsAssignmentRelated(event string) bool {
	switch event {
	case "taskAssigneeUpdated", "taskCreated":
		return true
	case "taskUpdated":
		return true
	default:
		return false
	}
}

// TaskUpdatedAssigneeChange returns true when event is taskUpdated and history
// mentions assignee add/remove (ClickUp also sends taskAssigneeUpdated separately).
func TaskUpdatedAssigneeChange(raw []byte) bool {
	var inner payloadWithHistory
	if err := json.Unmarshal(raw, &inner); err != nil || inner.Event != "taskUpdated" {
		return false
	}
	for _, item := range inner.HistoryItems {
		f := strings.TrimSpace(item.Field)
		if f == "assignee_add" || f == "assignee_remove" {
			return true
		}
	}
	return false
}

// AssigneeAddMatchesUser returns true when there is an assignee_add history item
// whose after.id matches wantUserID (ClickUp user id as string, e.g. "184").
// If wantUserID is empty, always returns true.
func AssigneeAddMatchesUser(raw []byte, wantUserID string) bool {
	wantUserID = strings.TrimSpace(wantUserID)
	if wantUserID == "" {
		return true
	}
	var inner payloadWithHistory
	if err := json.Unmarshal(raw, &inner); err != nil {
		return false
	}
	for _, item := range inner.HistoryItems {
		if strings.TrimSpace(item.Field) != "assignee_add" {
			continue
		}
		var after struct {
			ID interface{} `json:"id"`
		}
		if err := json.Unmarshal(item.After, &after); err != nil {
			continue
		}
		if afterIDToString(after.ID) == wantUserID {
			return true
		}
	}
	return false
}

func afterIDToString(id interface{}) string {
	switch v := id.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return fmt.Sprint(v)
	}
}
