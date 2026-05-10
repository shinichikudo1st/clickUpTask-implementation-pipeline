package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

const defaultResendAPIBase = "https://api.resend.com"

// ResendEmailService sends via Resend HTTP API (https://resend.com/docs/api-reference/emails/send-email).
type ResendEmailService struct {
	apiKey       string
	defaultFrom  string
	defaultTo    string
	baseURL      string
	maxAttach    int
	httpClient   *http.Client
}

// NewResendEmailService builds a Resend-backed sender from config (EMAIL_* fields).
func NewResendEmailService(cfg *config.Config) (*ResendEmailService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.EmailAPIBaseURL), "/")
	if base == "" {
		base = defaultResendAPIBase
	}
	return &ResendEmailService{
		apiKey:      cfg.EmailAPIKey,
		defaultFrom: cfg.EmailFrom,
		defaultTo:   cfg.EmailTo,
		baseURL:     base,
		maxAttach:   cfg.MaxEmailAttachmentBytes(),
		httpClient:  &http.Client{Timeout: 45 * time.Second},
	}, nil
}

type resendEmailPayload struct {
	From        string              `json:"from"`
	To          []string            `json:"to"`
	Subject     string              `json:"subject"`
	Text        string              `json:"text"`
	HTML        string              `json:"html"`
	Attachments []resendAttachment `json:"attachments,omitempty"`
}

type resendAttachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"content_type,omitempty"`
}

// SendMilestonePlan POSTs /emails with optional base64 markdown attachment.
func (r *ResendEmailService) SendMilestonePlan(ctx context.Context, in SendMilestoneInput) error {
	in2, attach, err := PrepareSendInput(in, r.maxAttach)
	if err != nil {
		return err
	}
	from := strings.TrimSpace(in2.From)
	if from == "" {
		from = r.defaultFrom
	}
	to := strings.TrimSpace(in2.To)
	if to == "" {
		to = r.defaultTo
	}
	if from == "" || to == "" {
		return fmt.Errorf("resend: from and to are required")
	}

	text, htmlBody := buildBodies(in2, attach)
	subj := subjectFor(in2)

	payload := resendEmailPayload{
		From:    from,
		To:      []string{to},
		Subject: subj,
		Text:    text,
		HTML:    htmlBody,
	}
	if attach && len(in2.MarkdownBody) > 0 {
		fn := strings.TrimSpace(in2.FileName)
		if fn == "" {
			fn = "milestone.md"
		}
		payload.Attachments = []resendAttachment{{
			Filename:    fn,
			Content:     base64.StdEncoding.EncodeToString(in2.MarkdownBody),
			ContentType: "text/markdown; charset=utf-8",
		}}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	u := r.baseURL + "/emails"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend: status %d: %s", resp.StatusCode, truncateErr(string(body), 500))
	}
	return nil
}

func truncateErr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
