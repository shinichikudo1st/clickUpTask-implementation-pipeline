package storage

import (
	"strings"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

func TestNewFromConfig_nil(t *testing.T) {
	_, err := NewFromConfig(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewFromConfig_localMissingDir(t *testing.T) {
	_, err := NewFromConfig(&config.Config{StorageBackend: "local", StorageLocalDir: ""})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewFromConfig_autoMissing(t *testing.T) {
	_, err := NewFromConfig(&config.Config{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "STORAGE") {
		t.Fatalf("error: %v", err)
	}
}

func TestNewFromConfig_supabaseMissingCreds(t *testing.T) {
	_, err := NewFromConfig(&config.Config{StorageBackend: "supabase"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewFromConfig_autoLocal(t *testing.T) {
	cfg := &config.Config{
		StorageBackend:  "auto",
		StorageLocalDir: t.TempDir(),
	}
	b, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b.(*LocalBlobStore); !ok {
		t.Fatalf("want LocalBlobStore got %T", b)
	}
}

func TestNewFromConfig_supabase(t *testing.T) {
	cfg := &config.Config{
		StorageBackend: "supabase",
		SupabaseURL:    "https://example.supabase.co",
		SupabaseKey:    "key",
	}
	b, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b.(*SupabaseBlobStore); !ok {
		t.Fatalf("want SupabaseBlobStore got %T", b)
	}
}
