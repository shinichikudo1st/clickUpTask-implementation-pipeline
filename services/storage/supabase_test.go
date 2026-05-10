package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
)

func TestSupabaseBlobStore_uploadDownloadSign(t *testing.T) {
	var uploaded []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/storage/v1/object/sign/"):
			_ = json.NewEncoder(w).Encode(map[string]string{
				"signedURL": "/object/sign/milestone-plans/t1/g1/file.md?token=test",
			})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/storage/v1/object/"):
			uploaded, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20))
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/storage/v1/object/"):
			_, _ = w.Write(uploaded)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s, err := NewSupabaseBlobStore(&config.Config{
		SupabaseURL: srv.URL,
		SupabaseKey: "service-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	content := []byte("# md\n")
	if err := s.Upload(ctx, "milestone-plans", "t1/g1/file.md", content, "text/markdown"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Download(ctx, "milestone-plans", "t1/g1/file.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("download %q", got)
	}
	u, err := s.SignedDownloadURL(ctx, "milestone-plans", "t1/g1/file.md", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u, "/object/sign/") || !strings.Contains(u, "token=test") {
		t.Fatalf("unexpected signed url %q", u)
	}
}

func TestMilestoneBucketName(t *testing.T) {
	if MilestoneBucketName(&config.Config{Bucket: "custom"}) != "custom" {
		t.Fatal()
	}
	if MilestoneBucketName(&config.Config{}) != DefaultMilestoneBucket {
		t.Fatal()
	}
}
