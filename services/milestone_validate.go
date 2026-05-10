package services

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/internal/checksum"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/internal/slug"
)

var (
	reOpenAIKey   = regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`)
	reStripeKey   = regexp.MustCompile(`(?i)(pk|sk)_(live|test)_[a-zA-Z0-9]{10,}`)
	reBearer      = regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9._\-]{24,}`)
	reLongHex     = regexp.MustCompile(`\b[0-9a-fA-F]{40,}\b`)
	reJWT         = regexp.MustCompile(`eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+`)
	reSupabaseSRV = regexp.MustCompile(`(?i)service_role|supabase.*key`)
	rePEMKey      = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
	reAWSKey      = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
)

// ValidateGeneratedMarkdown checks structure and rejects secret-like content.
func ValidateGeneratedMarkdown(md string) error {
	md = NormalizeNewlines(strings.TrimSpace(md))
	if md == "" {
		return fmt.Errorf("empty markdown")
	}
	lines := strings.Split(md, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return fmt.Errorf("must start with '# ' title line")
	}
	body := md
	for _, name := range []string{
		"Objective",
		"Recommended Approach",
		"Architecture",
		"Environment Variables",
		"Phases",
		"Master Checklist",
	} {
		if err := requireH2(body, name); err != nil {
			return err
		}
	}
	if err := scanSecrets(md); err != nil {
		return err
	}
	return nil
}

func requireH2(md, name string) error {
	needle := "## " + name
	if !strings.Contains(md, needle) {
		return fmt.Errorf("missing required section %q", needle)
	}
	return nil
}

func scanSecrets(md string) error {
	if reOpenAIKey.MatchString(md) {
		return fmt.Errorf("output looks like it contains an OpenAI-style API key")
	}
	if reStripeKey.MatchString(md) {
		return fmt.Errorf("output looks like it contains a Stripe-style key")
	}
	if reBearer.MatchString(md) {
		return fmt.Errorf("output looks like it contains a Bearer token")
	}
	if reJWT.MatchString(md) {
		return fmt.Errorf("output looks like it contains a JWT")
	}
	if reSupabaseSRV.MatchString(md) && reLongHex.MatchString(md) {
		return fmt.Errorf("output may contain a Supabase/service credential pattern")
	}
	if rePEMKey.MatchString(md) {
		return fmt.Errorf("output looks like it contains a PEM private key block")
	}
	if reAWSKey.MatchString(md) {
		return fmt.Errorf("output looks like it contains an AWS access key id")
	}
	// Long hex alone can be false positive; only flag if line looks like KEY=value
	for _, line := range strings.Split(md, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") ||
			strings.Contains(lower, "secret") && strings.Contains(lower, "=") {
			if reLongHex.MatchString(line) && len(strings.TrimSpace(line)) > 50 {
				return fmt.Errorf("output may contain a secret assignment")
			}
		}
	}
	return nil
}

// SHA256Hex returns lowercase hex SHA-256 of the UTF-8 bytes of s.
func SHA256Hex(s string) string {
	return checksum.Of([]byte(s))
}

// NormalizeNewlines converts CRLF and lone CR to LF for stable validation and hashing.
func NormalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// StripMarkdownFences removes optional ```markdown ... ``` wrapper from model output.
func StripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return NormalizeNewlines(s)
	}
	rest := strings.TrimSpace(s[3:])
	idx := strings.IndexByte(rest, '\n')
	if idx >= 0 {
		first := strings.TrimSpace(rest[:idx])
		// Language line (e.g. markdown) before body; headings start with # on that line only if no lang
		if first != "" && !strings.HasPrefix(first, "#") {
			rest = strings.TrimSpace(rest[idx+1:])
		}
	}
	if i := strings.LastIndex(rest, "```"); i >= 0 {
		rest = rest[:i]
	}
	return NormalizeNewlines(strings.TrimSpace(rest))
}

// MilestoneFileName returns "{taskID}-{slug}-milestone.md" with safe components.
func MilestoneFileName(taskID, taskTitle string) string {
	id := sanitizeFilePart(taskID, 48)
	s := slug.For(taskTitle)
	name := id + "-" + s + "-milestone.md"
	if len(name) > 200 {
		name = name[:200]
		// avoid cutting mid-extension
		if !strings.HasSuffix(name, ".md") {
			name = strings.TrimRight(name, "-_") + ".md"
		}
	}
	return name
}

func sanitizeFilePart(id string, max int) string {
	var b strings.Builder
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	s := strings.Trim(b.String(), "-_")
	if s == "" {
		s = "task"
	}
	if len(s) > max {
		s = s[:max]
	}
	return s
}
