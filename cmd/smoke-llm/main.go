// Command smoke-llm runs one real OpenAI milestone generation (uses LLM_* from env / .env).
//
// Usage (from repo clickUpTask-implementation-pipeline):
//
//	go run ./cmd/smoke-llm -context test/fixtures/smoke_task_context.example.json
//	go run ./cmd/smoke-llm -clickup-task <task_id> -comments
//	go run ./cmd/smoke-llm -clickup-task <task_id> -dump-context > my-task.json
//
// Requires LLM_API_KEY (and optional LLM_MODEL, LLM_API_BASE_URL). -clickup-task needs CLICKUP_API_TOKEN.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services"
)

func main() {
	var (
		contextPath   = flag.String("context", "", "path to JSON file describing models.TaskContext (see test/fixtures/smoke_task_context.example.json)")
		clickupTaskID = flag.String("clickup-task", "", "ClickUp task id to fetch (requires CLICKUP_API_TOKEN)")
		withComments  = flag.Bool("comments", false, "with -clickup-task: attach GetTaskComments JSON to task context")
		dumpContext   = flag.Bool("dump-context", false, "with -clickup-task: print JSON context to stdout and exit (no LLM call)")
		outPath       = flag.String("out", "", "if set, write generated markdown to this file; otherwise print to stdout")
	)
	flag.Parse()

	if (*contextPath == "" && *clickupTaskID == "") || (*contextPath != "" && *clickupTaskID != "") {
		fmt.Fprintln(os.Stderr, "exactly one of -context <file.json> or -clickup-task <id> is required")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var task models.TaskContext

	if *contextPath != "" {
		b, err := os.ReadFile(*contextPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read context:", err)
			os.Exit(1)
		}
		task, err = parseTaskContextFile(b)
		if err != nil {
			fmt.Fprintln(os.Stderr, "parse context:", err)
			os.Exit(1)
		}
	} else {
		cu, err := services.NewClickUpClient(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "clickup client:", err)
			os.Exit(1)
		}
		tc, err := cu.GetTask(ctx, *clickupTaskID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "get task:", err)
			os.Exit(1)
		}
		task = *tc
		if *withComments {
			comments, err := cu.GetTaskComments(ctx, task.TaskID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "get comments:", err)
				os.Exit(1)
			}
			task.CommentsJSON = comments
		}
		if *dumpContext {
			if err := json.NewEncoder(os.Stdout).Encode(taskContextFileFrom(task)); err != nil {
				fmt.Fprintln(os.Stderr, "encode:", err)
				os.Exit(1)
			}
			return
		}
	}

	gen, err := services.NewOpenAIGenerator(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generator:", err)
		os.Exit(1)
	}

	plan, err := gen.Generate(ctx, task)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "model=%s prompt=%s gen=%s file=%s sha256=%s\n",
		plan.Model, plan.PromptVersion, plan.GenerationVersion, plan.FileName, plan.MarkdownSHA256)

	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(plan.Markdown), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write out:", err)
			os.Exit(1)
		}
		return
	}
	if _, err := os.Stdout.WriteString(plan.Markdown); err != nil {
		fmt.Fprintln(os.Stderr, "write stdout:", err)
		os.Exit(1)
	}
	if !endsWithNewline(plan.Markdown) {
		_, _ = os.Stdout.WriteString("\n")
	}
}

func endsWithNewline(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '\n'
}

// taskContextFile mirrors models.TaskContext with JSON-friendly optional raw payloads.
type taskContextFile struct {
	TaskID           string            `json:"task_id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Status           string            `json:"status"`
	Priority         string            `json:"priority"`
	Assignees        []models.AssigneeRef `json:"assignees"`
	ListID           string            `json:"list_id"`
	ListName         string            `json:"list_name"`
	FolderID         string            `json:"folder_id"`
	FolderName       string            `json:"folder_name"`
	SpaceID          string            `json:"space_id"`
	SpaceName        string            `json:"space_name"`
	TeamID           string            `json:"team_id"`
	URL              string            `json:"url"`
	DateCreatedMs    string            `json:"date_created_ms"`
	DateUpdatedMs    string            `json:"date_updated_ms"`
	DueDateMs        string            `json:"due_date_ms"`
	CustomFieldsJSON json.RawMessage   `json:"custom_fields_json,omitempty"`
	RawTaskJSON      json.RawMessage   `json:"raw_task_json,omitempty"`
	CommentsJSON     json.RawMessage   `json:"comments_json,omitempty"`
}

func parseTaskContextFile(b []byte) (models.TaskContext, error) {
	var f taskContextFile
	if err := json.Unmarshal(b, &f); err != nil {
		return models.TaskContext{}, err
	}
	return models.TaskContext{
		TaskID:           f.TaskID,
		Name:             f.Name,
		Description:      f.Description,
		Status:           f.Status,
		Priority:         f.Priority,
		Assignees:        f.Assignees,
		CustomFieldsJSON: bytesOrNil(f.CustomFieldsJSON),
		ListID:           f.ListID,
		ListName:         f.ListName,
		FolderID:         f.FolderID,
		FolderName:       f.FolderName,
		SpaceID:          f.SpaceID,
		SpaceName:        f.SpaceName,
		TeamID:           f.TeamID,
		URL:              f.URL,
		DateCreatedMs:    f.DateCreatedMs,
		DateUpdatedMs:    f.DateUpdatedMs,
		DueDateMs:        f.DueDateMs,
		RawTaskJSON:      bytesOrNil(f.RawTaskJSON),
		CommentsJSON:     bytesOrNil(f.CommentsJSON),
	}, nil
}

func taskContextFileFrom(t models.TaskContext) taskContextFile {
	return taskContextFile{
		TaskID:           t.TaskID,
		Name:             t.Name,
		Description:      t.Description,
		Status:           t.Status,
		Priority:         t.Priority,
		Assignees:        t.Assignees,
		ListID:           t.ListID,
		ListName:         t.ListName,
		FolderID:         t.FolderID,
		FolderName:       t.FolderName,
		SpaceID:          t.SpaceID,
		SpaceName:        t.SpaceName,
		TeamID:           t.TeamID,
		URL:              t.URL,
		DateCreatedMs:    t.DateCreatedMs,
		DateUpdatedMs:    t.DateUpdatedMs,
		DueDateMs:        t.DueDateMs,
		CustomFieldsJSON: t.CustomFieldsJSON,
		RawTaskJSON:      t.RawTaskJSON,
		CommentsJSON:     t.CommentsJSON,
	}
}

func bytesOrNil(r json.RawMessage) []byte {
	if len(r) == 0 {
		return nil
	}
	return append([]byte(nil), r...)
}
