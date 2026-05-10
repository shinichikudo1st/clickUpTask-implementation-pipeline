package slug

import (
	"strings"
	"testing"
)

func TestFor(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "task"},
		{"   ", "task"},
		{"Hello World", "hello-world"},
		{"API  /  v2  !!", "api-v2"},
		{"___weird___", "weird"},
	}
	for _, tt := range tests {
		if got := For(tt.in); got != tt.want {
			t.Errorf("For(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestForMax(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := ForMax(long, 20)
	if len(got) > 20 {
		t.Fatalf("len %d > 20", len(got))
	}
	if got == "" {
		t.Fatal("empty slug")
	}
}
