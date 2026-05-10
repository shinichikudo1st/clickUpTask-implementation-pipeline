package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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
	ClickUpAPIBaseURL    string

	// Phase 10: optional poller (CLI or in-process ticker) for missed webhooks / local dev.
	ClickUpPollerEnabled    bool
	ClickUpPollIntervalSec  int // >0 with PollerEnabled starts a background ticker in main.
	ClickUpPollerLookbackH  int // first-run window when watermark is still at epoch

	LLMProvider   string
	LLMAPIKey     string
	LLMModel      string
	LLMAPIBaseURL string

	EmailProvider             string
	EmailAPIKey               string
	EmailFrom                 string
	EmailTo                   string
	EmailAPIBaseURL           string // optional; default https://api.resend.com for Resend
	EmailMaxAttachmentBytes   int    // optional; max markdown bytes to attach (default 450000)

	AppBaseURL string

	// Phase 6: object storage for generated markdown (local dir or Supabase Storage).
	StorageBackend      string // STORAGE_BACKEND: local | supabase | empty (see storage.NewFromConfig)
	StorageLocalDir     string // STORAGE_LOCAL_DIR: writable directory for local backend
	SignedURLTTLSeconds int    // SIGNED_URL_TTL_SECONDS: signed URL lifetime (Supabase); 0 → default 900s
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
		ClickUpAPIBaseURL:    strings.TrimSpace(os.Getenv("CLICKUP_API_BASE_URL")),
		ClickUpPollerEnabled:   boolFromEnv("CLICKUP_POLLER_ENABLED", false),
		ClickUpPollIntervalSec: intFromEnv("CLICKUP_POLL_INTERVAL_SECONDS", 0),
		ClickUpPollerLookbackH: intFromEnv("CLICKUP_POLLER_LOOKBACK_HOURS", 168),
		LLMProvider:          strings.TrimSpace(os.Getenv("LLM_PROVIDER")),
		LLMAPIKey:            strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		LLMModel:             strings.TrimSpace(os.Getenv("LLM_MODEL")),
		LLMAPIBaseURL:        strings.TrimSpace(os.Getenv("LLM_API_BASE_URL")),
		EmailProvider:             strings.TrimSpace(os.Getenv("EMAIL_PROVIDER")),
		EmailAPIKey:               strings.TrimSpace(os.Getenv("EMAIL_API_KEY")),
		EmailFrom:                 strings.TrimSpace(os.Getenv("EMAIL_FROM")),
		EmailTo:                   strings.TrimSpace(os.Getenv("EMAIL_TO")),
		EmailAPIBaseURL:           strings.TrimSpace(os.Getenv("EMAIL_API_BASE_URL")),
		EmailMaxAttachmentBytes:   intFromEnv("EMAIL_MAX_ATTACHMENT_BYTES", 0),
		AppBaseURL:                strings.TrimSpace(os.Getenv("APP_BASE_URL")),
		StorageBackend:       strings.TrimSpace(os.Getenv("STORAGE_BACKEND")),
		StorageLocalDir:      strings.TrimSpace(os.Getenv("STORAGE_LOCAL_DIR")),
		SignedURLTTLSeconds:  intFromEnv("SIGNED_URL_TTL_SECONDS", 0),
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

	if cfg.ClickUpAPIBaseURL != "" {
		if err := validateHTTPURL("CLICKUP_API_BASE_URL", cfg.ClickUpAPIBaseURL); err != nil {
			return nil, err
		}
	}

	if cfg.ClickUpPollIntervalSec < 0 {
		return nil, errors.New("CLICKUP_POLL_INTERVAL_SECONDS must be >= 0")
	}
	if cfg.ClickUpPollIntervalSec > 0 && cfg.ClickUpPollIntervalSec < 30 {
		return nil, errors.New("CLICKUP_POLL_INTERVAL_SECONDS must be at least 30 when non-zero")
	}
	if cfg.ClickUpPollerLookbackH < 1 || cfg.ClickUpPollerLookbackH > 24*365 {
		return nil, errors.New("CLICKUP_POLLER_LOOKBACK_HOURS must be between 1 and 8760")
	}

	if cfg.LLMAPIBaseURL != "" {
		if err := validateHTTPURL("LLM_API_BASE_URL", cfg.LLMAPIBaseURL); err != nil {
			return nil, err
		}
	}

	if cfg.StorageBackend != "" {
		switch strings.ToLower(cfg.StorageBackend) {
		case "local", "supabase", "auto":
		default:
			return nil, fmt.Errorf("STORAGE_BACKEND must be local, supabase, or auto, got %q", cfg.StorageBackend)
		}
	}

	if cfg.StorageLocalDir != "" {
		if err := validateLocalStorageDir(cfg.StorageLocalDir); err != nil {
			return nil, err
		}
	}

	if err := validateEmailConfig(cfg); err != nil {
		return nil, err
	}

	if cfg.EmailAPIBaseURL != "" {
		if err := validateHTTPURL("EMAIL_API_BASE_URL", cfg.EmailAPIBaseURL); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func validateEmailConfig(cfg *Config) error {
	p := strings.ToLower(strings.TrimSpace(cfg.EmailProvider))
	if p == "" {
		return nil
	}
	switch p {
	case "resend", "none", "noop":
	default:
		return fmt.Errorf("EMAIL_PROVIDER must be resend, none, or noop, got %q", cfg.EmailProvider)
	}
	if p == "resend" {
		if cfg.EmailAPIKey == "" {
			return errors.New("EMAIL_PROVIDER=resend requires EMAIL_API_KEY")
		}
		if cfg.EmailFrom == "" {
			return errors.New("EMAIL_PROVIDER=resend requires EMAIL_FROM")
		}
		if cfg.EmailTo == "" {
			return errors.New("EMAIL_PROVIDER=resend requires EMAIL_TO")
		}
	}
	return nil
}

func intFromEnv(key string, defaultVal int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}

func boolFromEnv(key string, defaultVal bool) bool {
	s := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if s == "" {
		return defaultVal
	}
	switch s {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func validateLocalStorageDir(dir string) error {
	// Reject obvious traversal; OS will still enforce real paths at runtime.
	if strings.Contains(dir, "..") {
		return errors.New("STORAGE_LOCAL_DIR must not contain '..'")
	}
	return nil
}

// MaxEmailAttachmentBytes caps markdown attachment size; larger bodies use download link only.
func (c *Config) MaxEmailAttachmentBytes() int {
	if c == nil || c.EmailMaxAttachmentBytes <= 0 {
		return 450_000
	}
	const maxCap = 10_000_000
	if c.EmailMaxAttachmentBytes > maxCap {
		return maxCap
	}
	return c.EmailMaxAttachmentBytes
}

// SignedURLTTL returns a bounded TTL for storage signed URLs (default 15m, min 1m, max 7d).
func (c *Config) SignedURLTTL() time.Duration {
	s := c.SignedURLTTLSeconds
	if s <= 0 {
		s = 900
	}
	const minSec, maxSec = 60, 604800
	if s < minSec {
		s = minSec
	}
	if s > maxSec {
		s = maxSec
	}
	return time.Duration(s) * time.Second
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
