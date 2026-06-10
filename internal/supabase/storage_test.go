package supabase

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStorageClientUploadObject(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/storage/v1/object/productos/00202158.jpg" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-upsert") != "true" {
			t.Fatalf("expected x-upsert header")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != "image-bytes" {
			t.Fatalf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewStorageClient(server.URL, "test-key")
	if err := client.UploadObject(context.Background(), "productos", "00202158.jpg", "image/jpeg", []byte("image-bytes")); err != nil {
		t.Fatalf("upload object: %v", err)
	}
}

func TestStorageClientPublicURL(t *testing.T) {
	t.Parallel()

	client := NewStorageClient("https://example.supabase.co", "test-key")
	url := client.PublicURL("productos", "00202158.jpg")
	expected := "https://example.supabase.co/storage/v1/object/public/productos/00202158.jpg"
	if url != expected {
		t.Fatalf("expected %q, got %q", expected, url)
	}
}

func TestStorageClientUploadObjectRLSErrorHint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"statusCode":"403","error":"Unauthorized","message":"new row violates row-level security policy"}`))
	}))
	defer server.Close()

	client := NewStorageClient(server.URL, "anon-like-key")
	err := client.UploadObject(context.Background(), "productos", "00201224.jpg", "image/jpeg", []byte("image-bytes"))
	if err == nil {
		t.Fatal("expected upload error")
	}
	if !strings.Contains(err.Error(), "SUPABASE_SERVICE_ROLE_KEY") {
		t.Fatalf("expected service role hint, got %v", err)
	}
}

func TestContentTypeFromExtension(t *testing.T) {
	t.Parallel()

	if got := ContentTypeFromExtension(`C:\Sys_Image\Fotos\Productos\BO-036.jpg`); got != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %s", got)
	}
	if got := ContentTypeFromExtension("file.PNG"); got != "image/png" {
		t.Fatalf("expected image/png, got %s", got)
	}
	if got := ContentTypeFromExtension("file.bin"); !strings.HasPrefix(got, "application/") {
		t.Fatalf("expected application/*, got %s", got)
	}
}
