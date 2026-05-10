package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultLLMModel      = "gpt-5.4-mini"
	maxJSONBytes         = 32000
)

// OpenAIGenerator calls OpenAI Chat Completions and validates milestone markdown.
type OpenAIGenerator struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewOpenAIGenerator builds a generator from config. LLM_API_KEY is required; LLM_MODEL defaults to gpt-5.4-mini.
func NewOpenAIGenerator(cfg *config.Config) (*OpenAIGenerator, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if strings.TrimSpace(cfg.LLMAPIKey) == "" {
		return nil, errors.New("LLM_API_KEY is required for generation")
	}
	prov := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	if prov != "" && prov != "openai" {
		return nil, fmt.Errorf("unsupported LLM_PROVIDER %q (only openai is supported)", cfg.LLMProvider)
	}
	if !strings.Contains(embeddedMilestonePrompt, "{{TASK_JSON}}") {
		return nil, errors.New("embedded milestone prompt is missing {{TASK_JSON}} placeholder")
	}
	model := strings.TrimSpace(cfg.LLMModel)
	if model == "" {
		model = defaultLLMModel
	}
	base := strings.TrimSpace(cfg.LLMAPIBaseURL)
	if base == "" {
		base = defaultOpenAIBaseURL
	}
	base = strings.TrimRight(base, "/")
	return &OpenAIGenerator{
		apiKey:  cfg.LLMAPIKey,
		model:   model,
		baseURL: base,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

var _ Generator = (*OpenAIGenerator)(nil)

// Generate calls the LLM and returns validated plan metadata.
func (g *OpenAIGenerator) Generate(ctx context.Context, task models.TaskContext) (*GeneratedMilestonePlan, error) {
	taskJSON, err := taskContextJSONForPrompt(task)
	if err != nil {
		return nil, err
	}
	idx := strings.Index(embeddedMilestonePrompt, "{{TASK_JSON}}")
	systemPart := strings.TrimSpace(embeddedMilestonePrompt[:idx])
	userPart := taskJSON

	// Newer models (e.g. GPT-5 family, o-series) reject max_tokens; use max_completion_tokens only.
	reqBody := map[string]interface{}{
		"model":                 g.model,
		"temperature":           0.2,
		"max_completion_tokens": 4096,
		"messages": []map[string]string{
			{"role": "system", "content": systemPart},
			{"role": "user", "content": userPart},
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	url := g.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai chat completions: status %d: %s", resp.StatusCode, truncateForErr(string(body), 512))
	}
	var parsed chatCompletionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}
	content = StripMarkdownFences(content)
	if err := ValidateGeneratedMarkdown(content); err != nil {
		return nil, fmt.Errorf("generated markdown failed validation: %w", err)
	}
	fileName := MilestoneFileName(task.TaskID, task.Name)
	return &GeneratedMilestonePlan{
		Markdown:          content,
		MarkdownSHA256:    SHA256Hex(content),
		FileName:          fileName,
		Model:             g.model,
		PromptVersion:     PromptVersionMilestoneV1,
		GenerationVersion: GenerationVersionV1,
	}, nil
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func taskContextJSONForPrompt(task models.TaskContext) (string, error) {
	assignees := make([]map[string]string, 0, len(task.Assignees))
	for _, a := range task.Assignees {
		assignees = append(assignees, map[string]string{
			"id":       a.ID,
			"username": a.Username,
			"email":    a.Email,
		})
	}
	m := map[string]interface{}{
		"task_id":            task.TaskID,
		"name":               task.Name,
		"description":        task.Description,
		"status":             task.Status,
		"priority":           task.Priority,
		"assignees":          assignees,
		"list_id":            task.ListID,
		"list_name":          task.ListName,
		"folder_id":          task.FolderID,
		"folder_name":        task.FolderName,
		"space_id":           task.SpaceID,
		"space_name":         task.SpaceName,
		"team_id":            task.TeamID,
		"url":                task.URL,
		"date_created_ms":    task.DateCreatedMs,
		"date_updated_ms":    task.DateUpdatedMs,
		"due_date_ms":        task.DueDateMs,
		"custom_fields_json": truncateJSONFragment(task.CustomFieldsJSON),
		"raw_task_json":      truncateJSONFragment(task.RawTaskJSON),
		"comments_json":      truncateJSONFragment(task.CommentsJSON),
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func truncateJSONFragment(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	s := string(b)
	if len(s) <= maxJSONBytes {
		return s
	}
	return s[:maxJSONBytes] + "\n... [truncated]"
}

func truncateForErr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
