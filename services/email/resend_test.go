package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

func TestResendEmailService_SendMilestonePlan(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"re_test"}`))
	}))
	defer srv.Close()

	svc, err := NewResendEmailService(&config.Config{
		EmailAPIKey:             "re_key",
		EmailFrom:               "from@example.com",
		EmailTo:                 "to@example.com",
		EmailAPIBaseURL:         srv.URL,
		EmailMaxAttachmentBytes: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.SendMilestonePlan(context.Background(), SendMilestoneInput{
		TaskName:     "My task",
		TaskURL:      "https://cu/t",
		FileName:     "m.md",
		MarkdownBody: []byte("# hello"),
		DownloadURL:  "https://signed",
	})
	if err != nil {
		t.Fatal(err)
	}
	atts, _ := gotBody["attachments"].([]interface{})
	if len(atts) != 1 {
		t.Fatalf("attachments: %v", gotBody["attachments"])
	}
	if gotBody["from"] != "from@example.com" {
		t.Fatalf("from: %v", gotBody["from"])
	}
}

func TestResendEmailService_noAttachWhenLarge(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	svc, err := NewResendEmailService(&config.Config{
		EmailAPIKey:             "k",
		EmailFrom:               "a@b.c",
		EmailTo:                 "d@e.f",
		EmailAPIBaseURL:         srv.URL,
		EmailMaxAttachmentBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	body := bytesRepeat('z', 500)
	err = svc.SendMilestonePlan(context.Background(), SendMilestoneInput{
		TaskName:     "t",
		MarkdownBody: body,
		DownloadURL:  "https://dl",
	})
	if err != nil {
		t.Fatal(err)
	}
	if atts, ok := gotBody["attachments"].([]interface{}); ok && len(atts) > 0 {
		t.Fatal("expected no attachments")
	}
	html, _ := gotBody["html"].(string)
	if !strings.Contains(html, "https://dl") {
		t.Fatalf("missing link in html: %s", html)
	}
}

func TestNewFromConfig_email(t *testing.T) {
	s, err := NewFromConfig(&config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(NoopEmailService); !ok {
		t.Fatalf("got %T", s)
	}
}
