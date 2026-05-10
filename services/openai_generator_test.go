package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

func TestNewOpenAIGenerator_config(t *testing.T) {
	_, err := NewOpenAIGenerator(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	_, err = NewOpenAIGenerator(&config.Config{LLMAPIKey: ""})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	_, err = NewOpenAIGenerator(&config.Config{LLMAPIKey: "k", LLMProvider: "anthropic"})
	if err == nil {
		t.Fatal("expected unsupported provider")
	}
	g, err := NewOpenAIGenerator(&config.Config{LLMAPIKey: "secret", LLMModel: ""})
	if err != nil {
		t.Fatal(err)
	}
	if g.model == "" {
		t.Fatal("default model")
	}
}

func TestOpenAIGenerator_Generate_happyPath(t *testing.T) {
	md := strings.TrimSpace(NormalizeNewlines(validMilestoneMarkdown()))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": md}},
			},
		})
	}))
	defer srv.Close()

	g, err := NewOpenAIGenerator(&config.Config{
		LLMAPIKey:     "test-key",
		LLMModel:      "gpt-test",
		LLMAPIBaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := g.Generate(context.Background(), models.TaskContext{
		TaskID: "9abc",
		Name:   "Ship Feature",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Markdown != md {
		t.Fatalf("markdown mismatch")
	}
	if out.MarkdownSHA256 != SHA256Hex(md) {
		t.Fatalf("checksum mismatch")
	}
	if !strings.HasSuffix(out.FileName, "-milestone.md") {
		t.Fatalf("filename %q", out.FileName)
	}
	if out.PromptVersion != PromptVersionMilestoneV1 || out.GenerationVersion != GenerationVersionV1 {
		t.Fatalf("versions %+v", out)
	}
}

func TestOpenAIGenerator_Generate_apiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusUnauthorized)
	}))
	defer srv.Close()
	g, err := NewOpenAIGenerator(&config.Config{
		LLMAPIKey:     "x",
		LLMAPIBaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Generate(context.Background(), models.TaskContext{TaskID: "1", Name: "n"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAIGenerator_Generate_invalidMarkdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "# only title"}},
			},
		})
	}))
	defer srv.Close()
	g, err := NewOpenAIGenerator(&config.Config{
		LLMAPIKey:     "x",
		LLMAPIBaseURL: srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.Generate(context.Background(), models.TaskContext{TaskID: "1", Name: "n"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestTaskContextJSONForPrompt_truncation(t *testing.T) {
	long := make([]byte, maxJSONBytes+100)
	for i := range long {
		long[i] = 'a'
	}
	s, err := taskContextJSONForPrompt(models.TaskContext{
		TaskID:      "t",
		Name:        "n",
		RawTaskJSON: long,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "[truncated]") {
		t.Fatalf("expected truncation marker in %d len string", len(s))
	}
}
