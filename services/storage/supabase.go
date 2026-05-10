package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

// SupabaseBlobStore uses Supabase Storage REST (service role key).
type SupabaseBlobStore struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
}

// NewSupabaseBlobStore requires https SUPABASE_URL and service role key.
func NewSupabaseBlobStore(cfg *config.Config) (*SupabaseBlobStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	u := strings.TrimRight(strings.TrimSpace(cfg.SupabaseURL), "/")
	key := strings.TrimSpace(cfg.SupabaseKey)
	if u == "" || key == "" {
		return nil, fmt.Errorf("supabase storage: SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY are required")
	}
	return &SupabaseBlobStore{
		baseURL:    u,
		serviceKey: key,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (s *SupabaseBlobStore) authHeaders() http.Header {
	h := make(http.Header)
	h.Set("Authorization", "Bearer "+s.serviceKey)
	h.Set("apikey", s.serviceKey)
	return h
}

// Upload upserts an object (x-upsert: true).
func (s *SupabaseBlobStore) Upload(ctx context.Context, bucket, objectPath string, content []byte, contentType string) error {
	if err := validateRelativeKey(objectPath); err != nil {
		return err
	}
	if strings.TrimSpace(bucket) == "" {
		return fmt.Errorf("bucket is required")
	}
	if contentType == "" {
		contentType = "text/markdown"
	}
	u := s.baseURL + "/storage/v1/object/" + objectPathURL(bucket, objectPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(content))
	if err != nil {
		return err
	}
	h := s.authHeaders()
	for k, vs := range h {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("supabase upload: status %d: %s", resp.StatusCode, truncateErrBody(body, 400))
	}
	return nil
}

// Download fetches object bytes using the service role (private bucket).
func (s *SupabaseBlobStore) Download(ctx context.Context, bucket, objectPath string) ([]byte, error) {
	if err := validateRelativeKey(objectPath); err != nil {
		return nil, err
	}
	u := s.baseURL + "/storage/v1/object/" + objectPathURL(bucket, objectPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	h := s.authHeaders()
	for k, vs := range h {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("supabase download: status %d: %s", resp.StatusCode, truncateErrBody(body, 400))
	}
	return body, nil
}

// SignedDownloadURL asks Supabase for a time-limited URL (expiresIn is in whole seconds per JS SDK).
func (s *SupabaseBlobStore) SignedDownloadURL(ctx context.Context, bucket, objectPath string, expiry time.Duration) (string, error) {
	if err := validateRelativeKey(objectPath); err != nil {
		return "", err
	}
	sec := int(expiry.Round(time.Second) / time.Second)
	if sec < 1 {
		sec = 60
	}
	u := s.baseURL + "/storage/v1/object/sign/" + objectPathURL(bucket, objectPath)
	payload := map[string]int{"expiresIn": sec}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	h := s.authHeaders()
	for k, vs := range h {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("supabase sign: status %d: %s", resp.StatusCode, truncateErrBody(body, 400))
	}
	var parsed struct {
		SignedURL string `json:"signedURL"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("supabase sign decode: %w", err)
	}
	if parsed.SignedURL == "" {
		return "", fmt.Errorf("supabase sign: empty signedURL in response")
	}
	if strings.HasPrefix(parsed.SignedURL, "http://") || strings.HasPrefix(parsed.SignedURL, "https://") {
		return parsed.SignedURL, nil
	}
	// Relative path under /storage/v1
	if strings.HasPrefix(parsed.SignedURL, "/") {
		return s.baseURL + "/storage/v1" + parsed.SignedURL, nil
	}
	return s.baseURL + "/storage/v1/" + strings.TrimPrefix(parsed.SignedURL, "/"), nil
}

func objectPathURL(bucket, objectPath string) string {
	segs := append([]string{bucket}, splitObjectPath(objectPath)...)
	return encodePathSegments(segs...)
}

func splitObjectPath(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func truncateErrBody(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
