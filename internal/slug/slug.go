package slug

import (
	"strings"
	"unicode"
)

const defaultMaxLen = 80

// For returns a filesystem-safe slug from a title (lowercase, hyphens, trimmed).
func For(title string) string {
	return ForMax(title, defaultMaxLen)
}

// ForMax is like For but caps the slug length at maxLen (minimum 8).
func ForMax(title string, maxLen int) string {
	if maxLen < 8 {
		maxLen = 8
	}
	s := strings.TrimSpace(strings.ToLower(title))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == ' ', r == '-', r == '_':
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "task"
	}
	if len(out) > maxLen {
		out = out[:maxLen]
		out = strings.TrimRight(out, "-")
	}
	if out == "" {
		return "task"
	}
	return out
}
