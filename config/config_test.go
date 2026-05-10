package config

import (
	"strings"
	"testing"
)

func TestLoad_MissingAPISecret(t *testing.T) {
	t.Setenv("API_SECRET", "")
	t.Setenv("PORT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API_SECRET") {
		t.Fatalf("error: %v", err)
	}
}

func TestLoad_APISecretTooShort(t *testing.T) {
	t.Setenv("API_SECRET", "short")
	t.Setenv("PORT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "at least") {
		t.Fatalf("error: %v", err)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("PORT", "99999")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "port") {
		t.Fatalf("error: %v", err)
	}
}

func TestLoad_PortNotInteger(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("PORT", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_ValidMinimal(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port: got %q", cfg.Port)
	}
	if cfg.APISecret != "longenough" {
		t.Fatalf("APISecret not loaded")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("PORT", "3000")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "3000" {
		t.Fatalf("Port: %q", cfg.Port)
	}
}

func TestLoad_SupabaseURLWithoutKey(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("SUPABASE_URL", "https://abc.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_SupabaseKeyWithoutURL(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "key")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_SupabaseHTTPSRequired(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("SUPABASE_URL", "http://abc.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "key")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("error: %v", err)
	}
}

func TestLoad_BucketWithoutSupabaseURL(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("SUPABASE_STORAGE_BUCKET", "plans")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_AppBaseURLInvalidScheme(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("APP_BASE_URL", "ftp://example.com")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_DatabaseURLDisallowSSLDisable(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("DATABASE_URL", "postgres://u:p@host:5432/db?sslmode=disable")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_DatabaseURLRequiresSSLMode(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("DATABASE_URL", "postgres://u:p@host:5432/db")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_ClickUpAPIBaseURLInvalidScheme(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("CLICKUP_API_BASE_URL", "ftp://example.com/api/v2")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_DatabaseURLAcceptRequire(t *testing.T) {
	t.Setenv("API_SECRET", "longenough")
	t.Setenv("DATABASE_URL", "postgres://u:p@host:5432/db?sslmode=require")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("expected database URL")
	}
}

func TestLoad_TrimsWhitespace(t *testing.T) {
	t.Setenv("API_SECRET", "  longenough  ")
	t.Setenv("PORT", "  9090  ")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APISecret != "longenough" {
		t.Fatalf("APISecret: %q", cfg.APISecret)
	}
	if cfg.Port != "9090" {
		t.Fatalf("Port: %q", cfg.Port)
	}
}
