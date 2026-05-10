package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const minAPISecretLen = 8

// Config holds all runtime settings loaded from the environment.
type Config struct {
	Port        string
	APISecret   string
	DatabaseURL string
	SupabaseURL string
	SupabaseKey string
	Bucket      string

	ClickUpToken         string
	ClickUpWebhookSecret string
	ClickUpTeamID        string
	ClickUpAssigneeID    string

	LLMProvider string
	LLMAPIKey   string
	LLMModel    string

	EmailProvider string
	EmailAPIKey   string
	EmailFrom     string
	EmailTo       string

	AppBaseURL string
}

// Load reads configuration from the process environment. Optional values are
// trimmed; required values are validated. godotenv.Load is best-effort for
// local development only (missing .env is not an error).
func Load() (*Config, error) {
	_ = godotenv.Load()

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	if err := validatePort(port); err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:                 port,
		APISecret:            strings.TrimSpace(os.Getenv("API_SECRET")),
		DatabaseURL:          strings.TrimSpace(os.Getenv("DATABASE_URL")),
		SupabaseURL:          strings.TrimSpace(os.Getenv("SUPABASE_URL")),
		SupabaseKey:          strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY")),
		Bucket:               strings.TrimSpace(os.Getenv("SUPABASE_STORAGE_BUCKET")),
		ClickUpToken:         strings.TrimSpace(os.Getenv("CLICKUP_API_TOKEN")),
		ClickUpWebhookSecret: strings.TrimSpace(os.Getenv("CLICKUP_WEBHOOK_SECRET")),
		ClickUpTeamID:        strings.TrimSpace(os.Getenv("CLICKUP_TEAM_ID")),
		ClickUpAssigneeID:    strings.TrimSpace(os.Getenv("CLICKUP_ASSIGNEE_USER_ID")),
		LLMProvider:          strings.TrimSpace(os.Getenv("LLM_PROVIDER")),
		LLMAPIKey:            strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		LLMModel:             strings.TrimSpace(os.Getenv("LLM_MODEL")),
		EmailProvider:        strings.TrimSpace(os.Getenv("EMAIL_PROVIDER")),
		EmailAPIKey:          strings.TrimSpace(os.Getenv("EMAIL_API_KEY")),
		EmailFrom:            strings.TrimSpace(os.Getenv("EMAIL_FROM")),
		EmailTo:              strings.TrimSpace(os.Getenv("EMAIL_TO")),
		AppBaseURL:           strings.TrimSpace(os.Getenv("APP_BASE_URL")),
	}

	if cfg.APISecret == "" {
		return nil, errors.New("API_SECRET is required")
	}
	if len(cfg.APISecret) < minAPISecretLen {
		return nil, fmt.Errorf("API_SECRET must be at least %d characters", minAPISecretLen)
	}

	if err := validateSupabasePair(cfg.SupabaseURL, cfg.SupabaseKey); err != nil {
		return nil, err
	}
	if cfg.SupabaseURL != "" {
		if err := validateHTTPSURL("SUPABASE_URL", cfg.SupabaseURL); err != nil {
			return nil, err
		}
	}

	if cfg.Bucket != "" && cfg.SupabaseURL == "" {
		return nil, errors.New("SUPABASE_STORAGE_BUCKET is set but SUPABASE_URL is empty")
	}

	if cfg.AppBaseURL != "" {
		if err := validateHTTPURL("APP_BASE_URL", cfg.AppBaseURL); err != nil {
			return nil, err
		}
	}

	if cfg.DatabaseURL != "" {
		if err := validateDatabaseTLS(cfg.DatabaseURL); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func validateDatabaseTLS(databaseURL string) error {
	dbLower := strings.ToLower(databaseURL)

	if strings.Contains(dbLower, "sslmode=disable") || strings.Contains(dbLower, "sslmode=allow") {
		return errors.New("DATABASE_URL must not use sslmode=disable or sslmode=allow; use sslmode=require or sslmode=verify-*")
	}

	if strings.Contains(dbLower, "sslmode=require") {
		return nil
	}
	if strings.Contains(dbLower, "sslmode=verify-") {
		return nil
	}

	return errors.New("DATABASE_URL must include sslmode=require (or sslmode=verify-*) for Supabase connections")
}

func validatePort(port string) error {
	n, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("PORT must be a valid integer: %w", err)
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535, got %d", n)
	}
	return nil
}

func validateSupabasePair(supabaseURL, serviceKey string) error {
	hasURL := supabaseURL != ""
	hasKey := serviceKey != ""
	if hasURL != hasKey {
		if hasURL {
			return errors.New("SUPABASE_URL is set but SUPABASE_SERVICE_ROLE_KEY is empty")
		}
		return errors.New("SUPABASE_SERVICE_ROLE_KEY is set but SUPABASE_URL is empty")
	}
	return nil
}

func validateHTTPSURL(name, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL", name)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%s must use https", name)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s must include a host", name)
	}
	return nil
}

func validateHTTPURL(name, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL", name)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", name)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s must include a host", name)
	}
	return nil
}
