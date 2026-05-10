package email

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
)

// EmailService sends milestone notification emails (Phase 7).
type EmailService interface {
	SendMilestonePlan(ctx context.Context, in SendMilestoneInput) error
}

// SendMilestoneInput carries everything needed for one milestone notification.
// MarkdownBody may be omitted when only a download link is used; at least one of
// MarkdownBody (non-empty) or DownloadURL must be set.
type SendMilestoneInput struct {
	To, From       string // if empty, provider uses config defaults where applicable
	TaskName       string
	TaskURL        string
	FileName       string // attachment name (.md)
	MarkdownBody   []byte
	DownloadURL    string // required when attachment is skipped (oversized) or as fallback link
}

func (in SendMilestoneInput) validate() error {
	if strings.TrimSpace(in.TaskName) == "" {
		return errors.New("task name is required")
	}
	if len(in.MarkdownBody) == 0 && strings.TrimSpace(in.DownloadURL) == "" {
		return errors.New("markdown body or download URL is required")
	}
	return nil
}

func subjectFor(in SendMilestoneInput) string {
	s := "Milestone plan: " + strings.TrimSpace(in.TaskName)
	const max = 900
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func buildBodies(in SendMilestoneInput, attach bool) (text, htmlBody string) {
	nameEsc := html.EscapeString(in.TaskName)
	urlEsc := html.EscapeString(strings.TrimSpace(in.TaskURL))
	linkEsc := html.EscapeString(strings.TrimSpace(in.DownloadURL))

	var b strings.Builder
	b.WriteString("Your ApexSuite-style milestone plan is ready.\n\n")
	b.WriteString("Task: ")
	b.WriteString(in.TaskName)
	b.WriteString("\n")
	if strings.TrimSpace(in.TaskURL) != "" {
		b.WriteString("ClickUp: ")
		b.WriteString(in.TaskURL)
		b.WriteString("\n")
	}
	if attach {
		b.WriteString("\nThe plan is attached as ")
		b.WriteString(in.FileName)
		b.WriteString(".\n")
	}
	if strings.TrimSpace(in.DownloadURL) != "" {
		b.WriteString("\nDownload: ")
		b.WriteString(in.DownloadURL)
		b.WriteString("\n")
	}
	text = b.String()

	var h strings.Builder
	h.WriteString("<p>Your ApexSuite-style milestone plan is ready.</p>")
	h.WriteString("<p><strong>Task:</strong> ")
	h.WriteString(nameEsc)
	h.WriteString("</p>")
	if urlEsc != "" {
		h.WriteString("<p><strong>ClickUp:</strong> <a href=\"")
		h.WriteString(urlEsc)
		h.WriteString("\">")
		h.WriteString(urlEsc)
		h.WriteString("</a></p>")
	}
	if attach {
		h.WriteString("<p>The plan is attached as <code>")
		h.WriteString(html.EscapeString(in.FileName))
		h.WriteString("</code>.</p>")
	}
	if linkEsc != "" {
		h.WriteString("<p><a href=\"")
		h.WriteString(linkEsc)
		h.WriteString("\">Download milestone markdown</a></p>")
	}
	htmlBody = h.String()
	return text, htmlBody
}

// ShouldAttachMarkdown returns true if markdown should be attached given size limits.
func ShouldAttachMarkdown(markdown []byte, maxBytes int) bool {
	if maxBytes <= 0 {
		return false
	}
	return len(markdown) > 0 && len(markdown) <= maxBytes
}

// PrepareSendInput decides attachment vs link-only from size rules.
func PrepareSendInput(in SendMilestoneInput, maxAttachBytes int) (SendMilestoneInput, bool, error) {
	if err := in.validate(); err != nil {
		return in, false, err
	}
	attach := ShouldAttachMarkdown(in.MarkdownBody, maxAttachBytes)
	if !attach && strings.TrimSpace(in.DownloadURL) == "" {
		return in, false, fmt.Errorf("markdown exceeds attachment limit (%d bytes) but DownloadURL is empty", maxAttachBytes)
	}
	return in, attach, nil
}
