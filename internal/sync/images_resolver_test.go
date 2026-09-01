package sync

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sycronizafhir/internal/config"
	"sycronizafhir/internal/db"
	"sycronizafhir/internal/supabase"
)

func TestImageResolverResolveProductRow(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/storage/v1/object/productos/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != "jpeg-content" {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "BO-036.jpg")
	if err := os.WriteFile(imagePath, []byte("jpeg-content"), 0o644); err != nil {
		t.Fatalf("write temp image: %v", err)
	}

	queue, err := db.NewSQLiteQueue(filepath.Join(tempDir, "queue.db"))
	if err != nil {
		t.Fatalf("open sqlite queue: %v", err)
	}
	defer queue.Close()

	cfg := config.Config{
		ImageSyncEnabled:       true,
		SupabaseURL:            server.URL,
		SupabaseServiceRole:    "test-key",
		StorageBucketProductos: "productos",
		ImageLocalBasePath:     tempDir,
	}
	resolver := NewImageResolver(cfg, queue, nil)
	resolver.storage = supabase.NewStorageClient(server.URL, "test-key")

	row := map[string]interface{}{
		"prod_id":     "00202158",
		"prod_imagen": imagePath,
	}
	if err := resolver.resolveProductRow(context.Background(), row); err != nil {
		t.Fatalf("resolve product row: %v", err)
	}

	publicURL, ok := row["prod_imagen"].(string)
	if !ok || !isRemoteImageURL(publicURL) {
		t.Fatalf("expected public url, got %#v", row["prod_imagen"])
	}
	if !strings.Contains(publicURL, "/storage/v1/object/public/productos/00202158.jpg") {
		t.Fatalf("unexpected public url: %s", publicURL)
	}
}

func TestIsLocalImageCachedDetectsOverwriteInFotosProductos(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "Fotos", "Productos", "PE2644.jpg")
	if err := os.MkdirAll(filepath.Dir(imagePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(imagePath, []byte("old-jpeg"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	queue, err := db.NewSQLiteQueue(filepath.Join(tempDir, "queue.db"))
	if err != nil {
		t.Fatalf("open sqlite queue: %v", err)
	}
	defer queue.Close()

	resolver := &ImageResolver{queue: queue, localBase: tempDir}
	ctx := context.Background()
	info, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := resolver.saveCachedURL(ctx, imageCacheKeyPrefix+strings.ToLower(imagePath), fileFingerprint([]byte("old-jpeg"), info.ModTime(), info.Size()), "https://example.supabase.co/storage/v1/object/public/productos/00201506.jpg"); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	if !resolver.IsLocalImageCached(ctx, imagePath) {
		t.Fatal("unchanged Fotos/Productos jpg should skip")
	}

	if err := os.WriteFile(imagePath, []byte("new-jpeg-better"), 0o644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if resolver.IsLocalImageCached(ctx, imagePath) {
		t.Fatal("recent overwrite in Fotos/Productos must re-upload")
	}
}
