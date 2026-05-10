package storage

import (
	"context"
	"errors"
	"time"
)

// DefaultMilestoneBucket is the Supabase Storage bucket name from Phase 6 milestone.
const DefaultMilestoneBucket = "milestone-plans"

// ErrSignedURLUnsupported means this backend cannot mint HTTP signed URLs (e.g. local disk).
var ErrSignedURLUnsupported = errors.New("storage: signed download URLs are not supported for this backend")

// BlobStore persists milestone markdown objects and can mint time-limited read URLs (Supabase).
type BlobStore interface {
	Upload(ctx context.Context, bucket, objectPath string, content []byte, contentType string) error
	Download(ctx context.Context, bucket, objectPath string) ([]byte, error)
	SignedDownloadURL(ctx context.Context, bucket, objectPath string, expiry time.Duration) (string, error)
}
