// Command smoke-email sends one test milestone-style email via Resend (uses EMAIL_* from .env).
//
// Usage (from repo clickUpTask-implementation-pipeline):
//
//	go run ./cmd/smoke-email
//	go run ./cmd/smoke-email -dry-run
//	go run ./cmd/smoke-email -markdown path/to/small.md
//
// Requires EMAIL_PROVIDER=resend plus EMAIL_API_KEY, EMAIL_FROM, EMAIL_TO (see .env.example).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services/email"
)

const defaultSmokeMarkdown = `# Smoke test

This is a one-off email from **cmd/smoke-email** to verify Resend + Phase 7 configuration.

- [ ] If you received this, Phase 7 mail path works.
`

func main() {
	dryRun := flag.Bool("dry-run", false, "validate config and input only; do not call Resend")
	mdPath := flag.String("markdown", "", "optional path to a small .md file to attach instead of the default body")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}

	prov := strings.ToLower(strings.TrimSpace(cfg.EmailProvider))
	if prov != "resend" {
		fmt.Fprintf(os.Stderr, "EMAIL_PROVIDER is %q — set EMAIL_PROVIDER=resend (with EMAIL_API_KEY, EMAIL_FROM, EMAIL_TO) to send a real test.\n", cfg.EmailProvider)
		os.Exit(1)
	}

	svc, err := email.NewFromConfig(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "email service:", err)
		os.Exit(1)
	}

	body := []byte(defaultSmokeMarkdown)
	if *mdPath != "" {
		b, err := os.ReadFile(*mdPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "read markdown:", err)
			os.Exit(1)
		}
		body = b
	}

	in := email.SendMilestoneInput{
		TaskName:       "Smoke email (Phase 7)",
		TaskURL:        "https://app.clickup.com",
		FileName:       "smoke-milestone.md",
		MarkdownBody:   body,
		DownloadURL:    "https://example.com/smoke-placeholder-download",
	}

	if *dryRun {
		if _, _, err := email.PrepareSendInput(in, cfg.MaxEmailAttachmentBytes()); err != nil {
			fmt.Fprintln(os.Stderr, "validate:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "dry-run ok: would send from %q to %q (subject contains task name).\n", cfg.EmailFrom, cfg.EmailTo)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if err := svc.SendMilestonePlan(ctx, in); err != nil {
		fmt.Fprintln(os.Stderr, "send:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "sent ok to %q from %q (check inbox and Resend dashboard → Logs).\n", cfg.EmailTo, cfg.EmailFrom)
}
