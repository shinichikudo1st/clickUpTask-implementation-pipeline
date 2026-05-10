package storage

import (
	"context"
	"errors"
	"testing"
)

func TestLocalBlobStore_roundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalBlobStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	bucket := "milestone-plans"
	key := "task1/gen/file.md"
	body := []byte("# hello\n")
	if err := s.Upload(ctx, bucket, key, body, "text/markdown"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Download(ctx, bucket, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q", got)
	}
	_, err = s.SignedDownloadURL(ctx, bucket, key, 0)
	if !errors.Is(err, ErrSignedURLUnsupported) {
		t.Fatalf("signed url: %v", err)
	}
}

func TestLocalBlobStore_rejectsEscape(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalBlobStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	err = s.Upload(ctx, "b", "../../outside", []byte("x"), "text/plain")
	if err == nil {
		t.Fatal("expected error")
	}
}
