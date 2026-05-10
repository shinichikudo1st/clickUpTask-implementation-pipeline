package storage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

// NewFromConfig picks a BlobStore implementation from STORAGE_BACKEND and related env.
//
// Rules:
//   - STORAGE_BACKEND=local → STORAGE_LOCAL_DIR required
//   - STORAGE_BACKEND=supabase → SUPABASE_URL + SUPABASE_SERVICE_ROLE_KEY required; bucket from
//     SUPABASE_STORAGE_BUCKET or default milestone-plans
//   - STORAGE_BACKEND empty or "auto": prefer local if STORAGE_LOCAL_DIR is set, else supabase if
//     Supabase URL+key are set; otherwise error
func NewFromConfig(cfg *config.Config) (BlobStore, error) {
	if cfg == nil {
		return nil, errors.New("storage: config is nil")
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.StorageBackend))
	if mode == "" {
		mode = "auto"
	}

	switch mode {
	case "auto":
		if cfg.StorageLocalDir != "" {
			return NewLocalBlobStore(cfg.StorageLocalDir)
		}
		if cfg.SupabaseURL != "" && cfg.SupabaseKey != "" {
			return NewSupabaseBlobStore(cfg)
		}
		return nil, fmt.Errorf("storage: set STORAGE_BACKEND=local with STORAGE_LOCAL_DIR, or configure SUPABASE_URL + SUPABASE_SERVICE_ROLE_KEY (optional STORAGE_BACKEND=supabase)")
	case "local":
		return NewLocalBlobStore(cfg.StorageLocalDir)
	case "supabase":
		if cfg.SupabaseURL == "" || cfg.SupabaseKey == "" {
			return nil, fmt.Errorf("storage: supabase backend requires SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY")
		}
		return NewSupabaseBlobStore(cfg)
	default:
		return nil, fmt.Errorf("storage: unknown STORAGE_BACKEND %q", cfg.StorageBackend)
	}
}

// MilestoneBucketName returns SUPABASE_STORAGE_BUCKET or the default milestone-plans.
func MilestoneBucketName(cfg *config.Config) string {
	if cfg == nil {
		return DefaultMilestoneBucket
	}
	if b := strings.TrimSpace(cfg.Bucket); b != "" {
		return b
	}
	return DefaultMilestoneBucket
}
