// Package safelog removes credential-like substrings before writing to logs (Phase 11).
package safelog

import "regexp"

var redactors = []*regexp.Regexp{
	// HTTP Authorization bearer tokens (ClickUp, OpenAI, Supabase JWT, etc.)
	// RE2 limits bounded repeats (e.g. max 1000); keep upper bound ≤ 1000.
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-+/=]{8,1000}\b`),
	// OpenAI-style secret keys
	regexp.MustCompile(`\bsk-[a-zA-Z0-9]{20,}\b`),
	// Stripe-style keys
	regexp.MustCompile(`(?i)\b(?:pk|sk)_(?:live|test)_[A-Za-z0-9]{10,}\b`),
	// JWT (three base64url-ish segments)
	regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`),
	// Common env assignment echo in errors
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*\S{8,256}`),
}

// Redact replaces secret-like substrings so log lines are safer to ship to aggregators.
func Redact(s string) string {
	if s == "" {
		return s
	}
	for _, re := range redactors {
		s = re.ReplaceAllString(s, `[REDACTED]`)
	}
	return s
}
