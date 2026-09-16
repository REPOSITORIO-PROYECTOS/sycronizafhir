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
	if err := resolver.saveCachedURL(ctx, imageCacheKey("00201506", imagePath), fileFingerprint([]byte("old-jpeg"), info.ModTime(), info.Size()), "https://example.supabase.co/storage/v1/object/public/productos/00201506.jpg"); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	if !resolver.IsLocalImageCached(ctx, "00201506", imagePath) {
		t.Fatal("unchanged Fotos/Productos jpg should skip")
	}
	if resolver.IsLocalImageCached(ctx, "00200772", imagePath) {
		t.Fatal("sibling prod_id sharing the same jpg must not inherit cache")
	}

	if err := os.WriteFile(imagePath, []byte("new-jpeg-better"), 0o644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if resolver.IsLocalImageCached(ctx, "00201506", imagePath) {
		t.Fatal("recent overwrite in Fotos/Productos must re-upload")
	}
}

func TestSharedLocalFileUploadsEachProdID(t *testing.T) {
	t.Parallel()

	var uploaded []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploaded = append(uploaded, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "PE0789.jpg")
	if err := os.WriteFile(imagePath, []byte("family-jpeg"), 0o644); err != nil {
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
	ctx := context.Background()

	for _, prodID := range []string{"01300108", "00200772"} {
		row := map[string]interface{}{
			"prod_id":     prodID,
			"prod_imagen": imagePath,
		}
		if err := resolver.resolveProductRow(ctx, row); err != nil {
			t.Fatalf("resolve %s: %v", prodID, err)
		}
		publicURL, _ := row["prod_imagen"].(string)
		want := "/storage/v1/object/public/productos/" + prodID + ".jpg"
		if !strings.Contains(publicURL, want) {
			t.Fatalf("prod %s url=%s want contains %s", prodID, publicURL, want)
		}
	}

	joined := strings.Join(uploaded, " ")
	if !strings.Contains(joined, "/storage/v1/object/productos/01300108.jpg") {
		t.Fatalf("missing first object upload: %v", uploaded)
	}
	if !strings.Contains(joined, "/storage/v1/object/productos/00200772.jpg") {
		t.Fatalf("sibling must upload own object, got %v", uploaded)
	}
}
