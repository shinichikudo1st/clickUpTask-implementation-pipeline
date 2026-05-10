package services

import (
	"strings"
	"testing"
)

func validMilestoneMarkdown() string {
	return `# Example Plan

## Objective

Do the thing.

## Recommended Approach

Incremental delivery.

## Architecture

Services and DB.

## Environment Variables

- API_SECRET
- DATABASE_URL

## Phases

### Phase 1 - Setup

#### Tasks

- [ ] Step one

#### Milestone 1 Checkpoint

- [ ] Review

## Master Checklist

- [ ] Final sign-off
`
}

func TestValidateGeneratedMarkdown_ok(t *testing.T) {
	if err := ValidateGeneratedMarkdown(validMilestoneMarkdown()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGeneratedMarkdown_okWithOptionalSections(t *testing.T) {
	md := `# Plan

## Objective

Ship it.

## Recommended Approach

**Decision:** Worker.

## Architecture

` + "```text\nA --> B\n```" + `

## API Contract

### GET /v1/x

Example.

## Environment Variables

` + "```text\nAPI_SECRET=\n```" + `

## Phases

### Phase 0 - Bootstrap

#### Tasks

- [ ] Init

#### Milestone 0 Checkpoint

- [ ] Runs

## Master Checklist

- [ ] Done
`
	if err := ValidateGeneratedMarkdown(md); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGeneratedMarkdown_errors(t *testing.T) {
	cases := []struct {
		name string
		md   string
	}{
		{"empty", ""},
		{"no_h1", "## Objective\n\nx"},
		{"missing_objective", "# T\n\n## Phases\n\nx\n\n## Master Checklist\n\nx"},
		{"missing_phases", "# T\n\n## Objective\n\nx\n\n## Recommended Approach\n\nx\n\n## Architecture\n\nx\n\n## Environment Variables\n\nx\n\n## Master Checklist\n\nx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateGeneratedMarkdown(tc.md); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestValidateGeneratedMarkdown_secrets(t *testing.T) {
	base := validMilestoneMarkdown()
	if err := ValidateGeneratedMarkdown(base + "\nsk-123456789012345678901234567890"); err == nil {
		t.Fatal("expected sk- rejection")
	}
	if err := ValidateGeneratedMarkdown(base + "\nBearer abcdefghijklmnopqrstuvwxyz0123456789"); err == nil {
		t.Fatal("expected bearer rejection")
	}
	if err := ValidateGeneratedMarkdown(base + "\n-----BEGIN RSA PRIVATE KEY-----\nMII"); err == nil {
		t.Fatal("expected PEM private key rejection")
	}
	if err := ValidateGeneratedMarkdown(base + "\nAKIA0123456789ABCDEF"); err == nil {
		t.Fatal("expected AWS access key id rejection")
	}
}

func TestStripMarkdownFences(t *testing.T) {
	inner := validMilestoneMarkdown()
	wrapped := "```markdown\n" + inner + "\n```"
	if got := StripMarkdownFences(wrapped); strings.TrimSpace(got) != strings.TrimSpace(inner) {
		t.Fatalf("got %q", got)
	}
	if got := StripMarkdownFences(inner); got != strings.TrimSpace(inner) {
		t.Fatalf("unwrap changed bare markdown")
	}
}

func TestMilestoneFileName(t *testing.T) {
	n := MilestoneFileName("abc-12", "My Cool Task!!")
	if !strings.HasSuffix(n, "-milestone.md") {
		t.Fatalf("suffix: %q", n)
	}
	if strings.ContainsAny(n, " !") {
		t.Fatalf("unsafe chars: %q", n)
	}
	n2 := MilestoneFileName("", "")
	if !strings.Contains(n2, "task") {
		t.Fatalf("empty ids: %q", n2)
	}
}

func TestSHA256Hex_deterministic(t *testing.T) {
	a := SHA256Hex("hello")
	b := SHA256Hex("hello")
	if a != b || len(a) != 64 {
		t.Fatalf("sha256 hex: %q %q", a, b)
	}
}
