package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

func testConfigWithClickUpToken(token string) *config.Config {
	return &config.Config{
		Port:         "8080",
		APISecret:    "longenough",
		ClickUpToken: token,
	}
}

func TestNewClickUpClient_requiresToken(t *testing.T) {
	t.Parallel()
	_, err := NewClickUpClient(testConfigWithClickUpToken(""))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetTask_success(t *testing.T) {
	t.Parallel()
	const taskJSON = `{
  "id": "9hx",
  "name": "Fix login bug",
  "text_content": "Body here",
  "description": "Desc",
  "status": {"status": "in progress"},
  "priority": {"priority": "normal"},
  "assignees": [{"id": 2772463, "username": "Alex", "email": "a@b.com"}],
  "custom_fields": [],
  "list": {"id": "15505202", "name": "Sprint Backlog"},
  "folder": {"id": "6992470", "name": "Mobile Squad"},
  "space": {"id": "7002367", "name": "Space A"},
  "team_id": "90161583571",
  "url": "https://app.clickup.com/t/9hx",
  "date_created": "1567780450202",
  "date_updated": "1567780450203",
  "due_date": "1508369194377"
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/task/9hx" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "pk_test" {
			t.Fatalf("auth header missing")
		}
		if r.URL.Query().Get("include_markdown_description") != "true" {
			t.Fatalf("query: %v", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(taskJSON))
	}))
	t.Cleanup(server.Close)

	client := &ClickUpClient{
		httpClient: server.Client(),
		baseURL:    strings.TrimSuffix(server.URL, "/") + "/api/v2",
		token:      "pk_test",
	}

	tc, err := client.GetTask(context.Background(), "9hx")
	if err != nil {
		t.Fatal(err)
	}
	if tc.TaskID != "9hx" || tc.Name != "Fix login bug" {
		t.Fatalf("task: %+v", tc)
	}
	if tc.Status != "in progress" || tc.Priority != "normal" {
		t.Fatalf("status/priority: %+v", tc)
	}
	if len(tc.Assignees) != 1 || tc.Assignees[0].ID != "2772463" {
		t.Fatalf("assignees: %+v", tc.Assignees)
	}
	if tc.ListID != "15505202" || tc.ListName != "Sprint Backlog" {
		t.Fatalf("list: %+v", tc)
	}
	if tc.TeamID != "90161583571" {
		t.Fatalf("team_id: %q", tc.TeamID)
	}
	if tc.URL == "" || tc.DateCreatedMs == "" {
		t.Fatalf("url/dates: %+v", tc)
	}
	if string(tc.RawTaskJSON) != taskJSON {
		t.Fatal("raw json not preserved")
	}
}

func TestGetTask_notFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"err":"missing"}`))
	}))
	t.Cleanup(server.Close)

	client := &ClickUpClient{
		httpClient: server.Client(),
		baseURL:    strings.TrimSuffix(server.URL, "/") + "/api/v2",
		token:      "pk_test",
	}

	_, err := client.GetTask(context.Background(), "missing")
	var he *ClickUpHTTPError
	if !errors.As(err, &he) || he.Code != "NOT_FOUND" {
		t.Fatalf("err: %v", err)
	}
}

func TestGetTask_rateLimitRetryAfterSeconds(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"err":"rate"}`))
	}))
	t.Cleanup(server.Close)

	client := &ClickUpClient{
		httpClient: server.Client(),
		baseURL:    strings.TrimSuffix(server.URL, "/") + "/api/v2",
		token:      "pk_test",
	}

	_, err := client.GetTask(context.Background(), "x")
	var he *ClickUpHTTPError
	if !errors.As(err, &he) || he.Code != "RATE_LIMIT" {
		t.Fatalf("err: %v", err)
	}
	if he.RetryAfter != 3*time.Second {
		t.Fatalf("retry: %v", he.RetryAfter)
	}
}

func TestGetTask_unauthorized(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client := &ClickUpClient{
		httpClient: server.Client(),
		baseURL:    strings.TrimSuffix(server.URL, "/") + "/api/v2",
		token:      "bad",
	}

	_, err := client.GetTask(context.Background(), "9hx")
	var he *ClickUpHTTPError
	if !errors.As(err, &he) || he.Code != "UNAUTHORIZED" {
		t.Fatalf("err: %v", err)
	}
}

func TestGetTask_invalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{`))
	}))
	t.Cleanup(server.Close)

	client := &ClickUpClient{
		httpClient: server.Client(),
		baseURL:    strings.TrimSuffix(server.URL, "/") + "/api/v2",
		token:      "pk_test",
	}

	_, err := client.GetTask(context.Background(), "9hx")
	if err == nil || !strings.Contains(err.Error(), "decode task json") {
		t.Fatalf("err: %v", err)
	}
}

func TestGetTask_emptyID(t *testing.T) {
	t.Parallel()
	client := &ClickUpClient{httpClient: http.DefaultClient, baseURL: "http://x", token: "t"}
	_, err := client.GetTask(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetTaskComments_success(t *testing.T) {
	t.Parallel()
	const commentsJSON = `{"comments":[{"id":"c1","comment_text":"hi"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/task/9hx/comment" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(commentsJSON))
	}))
	t.Cleanup(server.Close)

	client := &ClickUpClient{
		httpClient: server.Client(),
		baseURL:    strings.TrimSuffix(server.URL, "/") + "/api/v2",
		token:      "pk_test",
	}

	raw, err := client.GetTaskComments(context.Background(), "9hx")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != commentsJSON {
		t.Fatalf("got %s", raw)
	}
}

func TestNewClickUpClient_usesDefaultBaseURL(t *testing.T) {
	t.Parallel()
	cfg := testConfigWithClickUpToken("pk_x")
	cfg.ClickUpAPIBaseURL = ""
	c, err := NewClickUpClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != defaultClickUpAPIBase {
		t.Fatalf("base: %q", c.baseURL)
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	if d := parseRetryAfter("10"); d != 10*time.Second {
		t.Fatalf("seconds: %v", d)
	}
	if d := parseRetryAfter(""); d != 0 {
		t.Fatalf("empty: %v", d)
	}
}

func TestNewClickUpClient_customBaseURL(t *testing.T) {
	t.Parallel()
	cfg := testConfigWithClickUpToken("pk_x")
	cfg.ClickUpAPIBaseURL = "https://example.com/api/v2"
	c, err := NewClickUpClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != "https://example.com/api/v2" {
		t.Fatalf("base: %q", c.baseURL)
	}
}

func TestClickUpClient_ListTeamTasksForAssignee_pagination(t *testing.T) {
	t.Parallel()
	var pageHits []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/team/9001/task" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("assignees[]") != "777" {
			t.Fatalf("assignees: %v", q)
		}
		page := q.Get("page")
		pageHits = append(pageHits, page)
		w.Header().Set("Content-Type", "application/json")
		var payload any
		switch page {
		case "0":
			tasks := make([]map[string]string, 100)
			for i := 0; i < 100; i++ {
				tasks[i] = map[string]string{"id": "p0-" + strconv.Itoa(i), "date_updated": "1700000000000"}
			}
			payload = map[string]any{"tasks": tasks}
		case "1":
			payload = map[string]any{"tasks": []map[string]string{{"id": "p1-last", "date_updated": "1700000000001"}}}
		default:
			payload = map[string]any{"tasks": []any{}}
		}
		b, _ := json.Marshal(payload)
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	client := &ClickUpClient{
		httpClient: server.Client(),
		baseURL:    strings.TrimSuffix(server.URL, "/") + "/api/v2",
		token:      "pk_test",
	}
	gt := time.UnixMilli(1699999999999)
	tasks, err := client.ListTeamTasksForAssignee(context.Background(), "9001", "777", gt)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 101 {
		t.Fatalf("len=%d", len(tasks))
	}
	if len(pageHits) != 2 || pageHits[0] != "0" || pageHits[1] != "1" {
		t.Fatalf("pages=%v", pageHits)
	}
}
