package safelog

import (
	"strings"
	"testing"
)

func TestRedact_bearer(t *testing.T) {
	t.Parallel()
	in := `auth failed: Bearer sk-not-real-but-long-token-here-please-redact`
	got := Redact(in)
	if strings.Contains(got, "sk-not-real") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("got %q", got)
	}
}

func TestRedact_openAIKey(t *testing.T) {
	t.Parallel()
	in := `leaked sk-12345678901234567890123456789012 in text`
	got := Redact(in)
	if strings.Contains(got, "sk-1234") {
		t.Fatalf("got %q", got)
	}
}

func TestRedact_stripe(t *testing.T) {
	t.Parallel()
	in := `key pk_live_123456789012345678901234567890`
	got := Redact(in)
	if strings.Contains(got, "pk_live") {
		t.Fatalf("got %q", got)
	}
}

func TestRedact_preservesShortStrings(t *testing.T) {
	t.Parallel()
	in := `hello world 401 unauthorized`
	if Redact(in) != in {
		t.Fatalf("changed: %q", Redact(in))
	}
}
