package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalBlobStore writes objects under a root directory: root/{bucket}/{objectPath}.
type LocalBlobStore struct {
	root string
}

// NewLocalBlobStore creates a filesystem-backed store. The root directory is created if missing.
func NewLocalBlobStore(root string) (*LocalBlobStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("local storage: STORAGE_LOCAL_DIR is required")
	}
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("local storage mkdir: %w", err)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("local storage abs root: %w", err)
	}
	return &LocalBlobStore{root: absRoot}, nil
}

func (l *LocalBlobStore) absPath(bucket, objectPath string) (string, error) {
	if err := validateRelativeKey(objectPath); err != nil {
		return "", err
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return "", errors.New("bucket is required")
	}
	if err := validateRelativeKey(bucket); err != nil {
		return "", fmt.Errorf("bucket: %w", err)
	}
	full := filepath.Join(l.root, bucket, filepath.FromSlash(objectPath))
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	prefix := l.root + string(os.PathSeparator)
	if absFull != l.root && !strings.HasPrefix(absFull, prefix) {
		return "", errors.New("resolved path escapes storage root")
	}
	return absFull, nil
}

// Upload writes content to disk (overwrites if present).
func (l *LocalBlobStore) Upload(ctx context.Context, bucket, objectPath string, content []byte, contentType string) error {
	_ = contentType
	_ = ctx
	dest, err := l.absPath(bucket, objectPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("local storage mkdir parent: %w", err)
	}
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("local storage write: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("local storage rename: %w", err)
	}
	return nil
}

// Download reads an object from disk.
func (l *LocalBlobStore) Download(ctx context.Context, bucket, objectPath string) ([]byte, error) {
	_ = ctx
	dest, err := l.absPath(bucket, objectPath)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("local storage: %w", err)
		}
		return nil, err
	}
	return b, nil
}

// SignedDownloadURL is not supported for local files.
func (l *LocalBlobStore) SignedDownloadURL(ctx context.Context, bucket, objectPath string, expiry time.Duration) (string, error) {
	_ = l
	_ = ctx
	_ = bucket
	_ = objectPath
	_ = expiry
	return "", ErrSignedURLUnsupported
}
