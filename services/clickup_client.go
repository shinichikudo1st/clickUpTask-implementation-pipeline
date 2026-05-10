package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

const defaultClickUpAPIBase = "https://api.clickup.com/api/v2"

const clickUpRequestTimeout = 30 * time.Second

// ClickUpClient calls ClickUp API v2 (tasks, comments).
type ClickUpClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClickUpClient builds a client from config. Requires CLICKUP_API_TOKEN.
func NewClickUpClient(cfg *config.Config) (*ClickUpClient, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	token := strings.TrimSpace(cfg.ClickUpToken)
	if token == "" {
		return nil, errors.New("CLICKUP_API_TOKEN is required")
	}

	base := strings.TrimSpace(cfg.ClickUpAPIBaseURL)
	base = strings.TrimSuffix(base, "/")
	if base == "" {
		base = defaultClickUpAPIBase
	}

	return &ClickUpClient{
		httpClient: &http.Client{Timeout: clickUpRequestTimeout},
		baseURL:    base,
		token:      token,
	}, nil
}

// GetTask fetches a single task and normalizes it into TaskContext.
// Query include_markdown_description=true improves description text for planning.
func (c *ClickUpClient) GetTask(ctx context.Context, taskID string) (*models.TaskContext, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("task_id is required")
	}

	u, err := url.Parse(c.baseURL + "/task/" + url.PathEscape(taskID))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("include_markdown_description", "true")
	u.RawQuery = q.Encode()

	raw, status, hdr, err := c.getBytes(ctx, u.String())
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, mapClickUpFailure(status, raw, hdr)
	}

	tc, err := normalizeTask(taskID, raw)
	if err != nil {
		return nil, err
	}
	return tc, nil
}

// GetTaskComments fetches task comments JSON (full API response body) for optional use in prompts.
func (c *ClickUpClient) GetTaskComments(ctx context.Context, taskID string) ([]byte, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("task_id is required")
	}
	u := c.baseURL + "/task/" + url.PathEscape(taskID) + "/comment"
	raw, status, hdr, err := c.getBytes(ctx, u)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, mapClickUpFailure(status, raw, hdr)
	}
	return raw, nil
}

func (c *ClickUpClient) getBytes(ctx context.Context, requestURL string) ([]byte, int, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, resp.Header, err
	}
	return body, resp.StatusCode, resp.Header, nil
}

func mapClickUpFailure(status int, body []byte, header http.Header) *ClickUpHTTPError {
	msg := strings.TrimSpace(string(body))
	if len(msg) > 500 {
		msg = msg[:500] + "…"
	}
	if msg == "" {
		msg = "clickup request failed"
	}

	err := &ClickUpHTTPError{StatusCode: status, Message: msg}
	switch status {
	case http.StatusNotFound:
		err.Code = "NOT_FOUND"
	case http.StatusUnauthorized:
		err.Code = "UNAUTHORIZED"
	case http.StatusForbidden:
		err.Code = "FORBIDDEN"
	case http.StatusTooManyRequests:
		err.Code = "RATE_LIMIT"
		if header != nil {
			err.RetryAfter = parseRetryAfter(header.Get("Retry-After"))
		}
	default:
		err.Code = "UPSTREAM_ERROR"
	}
	return err
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if sec, err := strconv.Atoi(value); err == nil && sec >= 0 {
		return time.Duration(sec) * time.Second
	}
	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

type taskDTO struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	TextContent  string          `json:"text_content"`
	Description  string          `json:"description"`
	Status       *namedDTO       `json:"status"`
	Priority     *priorityDTO    `json:"priority"`
	Assignees    []assigneeDTO   `json:"assignees"`
	CustomFields json.RawMessage `json:"custom_fields"`
	List         *locationDTO    `json:"list"`
	Folder       *locationDTO    `json:"folder"`
	Space        *locationDTO    `json:"space"`
	TeamID       json.RawMessage `json:"team_id"`
	URL          string          `json:"url"`
	DateCreated  string          `json:"date_created"`
	DateUpdated  string          `json:"date_updated"`
	DueDate      string          `json:"due_date"`
}

type namedDTO struct {
	Status string `json:"status"`
}

type priorityDTO struct {
	Priority string `json:"priority"`
}

type assigneeDTO struct {
	ID       interface{} `json:"id"`
	Username string      `json:"username"`
	Email    string      `json:"email"`
}

type locationDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func normalizeTask(requestedTaskID string, raw []byte) (*models.TaskContext, error) {
	var dto taskDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return nil, fmt.Errorf("decode task json: %w", err)
	}

	desc := strings.TrimSpace(dto.TextContent)
	if desc == "" {
		desc = strings.TrimSpace(dto.Description)
	}

	status := ""
	if dto.Status != nil {
		status = strings.TrimSpace(dto.Status.Status)
	}

	priority := ""
	if dto.Priority != nil {
		priority = strings.TrimSpace(dto.Priority.Priority)
	}

	assignees := make([]models.AssigneeRef, 0, len(dto.Assignees))
	for _, a := range dto.Assignees {
		assignees = append(assignees, models.AssigneeRef{
			ID:       idToString(a.ID),
			Username: strings.TrimSpace(a.Username),
			Email:    strings.TrimSpace(a.Email),
		})
	}

	custom := dto.CustomFields
	if custom == nil {
		custom = json.RawMessage("[]")
	}

	listID, listName := "", ""
	if dto.List != nil {
		listID = strings.TrimSpace(dto.List.ID)
		listName = strings.TrimSpace(dto.List.Name)
	}
	folderID, folderName := "", ""
	if dto.Folder != nil {
		folderID = strings.TrimSpace(dto.Folder.ID)
		folderName = strings.TrimSpace(dto.Folder.Name)
	}
	spaceID, spaceName := "", ""
	if dto.Space != nil {
		spaceID = strings.TrimSpace(dto.Space.ID)
		spaceName = strings.TrimSpace(dto.Space.Name)
	}

	taskID := strings.TrimSpace(dto.ID)
	if taskID == "" {
		taskID = requestedTaskID
	}

	teamID := strings.TrimSpace(rawJSONString(dto.TeamID))

	tc := &models.TaskContext{
		TaskID:           taskID,
		Name:             strings.TrimSpace(dto.Name),
		Description:      desc,
		Status:           status,
		Priority:         priority,
		Assignees:        assignees,
		CustomFieldsJSON: append([]byte(nil), custom...),
		ListID:           listID,
		ListName:         listName,
		FolderID:         folderID,
		FolderName:       folderName,
		SpaceID:          spaceID,
		SpaceName:        spaceName,
		TeamID:           teamID,
		URL:              strings.TrimSpace(dto.URL),
		DateCreatedMs:    strings.TrimSpace(dto.DateCreated),
		DateUpdatedMs:    strings.TrimSpace(dto.DateUpdated),
		DueDateMs:        strings.TrimSpace(dto.DueDate),
		RawTaskJSON:      append([]byte(nil), raw...),
	}
	return tc, nil
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatInt(int64(f), 10)
	}
	return strings.Trim(string(raw), `"`)
}

func idToString(id interface{}) string {
	switch v := id.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
