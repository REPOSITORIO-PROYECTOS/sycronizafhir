package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsLocalImagePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value string
		want  bool
	}{
		{`C:\Sys_Image\Fotos\Productos\BO-036.jpg`, true},
		{`\\server\share\image.jpg`, true},
		{"https://example.supabase.co/storage/v1/object/public/productos/a.jpg", false},
		{"", false},
		{`Fotos/Productos/BO-036.jpg`, true},
		{"BO-036.jpg", true},
		{"relative/path.jpg", true},
	}

	for _, tc := range cases {
		if got := isLocalImagePath(tc.value); got != tc.want {
			t.Fatalf("isLocalImagePath(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}

func TestBuildStorageObjectPath(t *testing.T) {
	t.Parallel()

	if got := buildStorageObjectPath("00202158", `C:\Sys_Image\Fotos\Productos\BO-036.jpg`); got != "00202158.jpg" {
		t.Fatalf("expected 00202158.jpg, got %s", got)
	}
	if got := buildStorageObjectPath("00202158", `C:\Sys_Image\Fotos\Productos\BO-036.PNG`); got != "00202158.png" {
		t.Fatalf("expected 00202158.png, got %s", got)
	}
}

func TestResolveLocalImagePathRelative(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "Fotos", "Productos")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	imagePath := filepath.Join(nestedDir, "BO-036.jpg")
	if err := os.WriteFile(imagePath, []byte("jpeg"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	got, err := resolveLocalImagePath(tempDir, "Fotos/Productos/BO-036.jpg")
	if err != nil {
		t.Fatalf("resolve relative forward slash: %v", err)
	}
	if got != imagePath {
		t.Fatalf("expected %q, got %q", imagePath, got)
	}

	barePath := filepath.Join(tempDir, "solo.jpg")
	if err := os.WriteFile(barePath, []byte("jpeg"), 0o644); err != nil {
		t.Fatalf("write bare image: %v", err)
	}
	got, err = resolveLocalImagePath(tempDir, "solo.jpg")
	if err != nil {
		t.Fatalf("resolve bare filename: %v", err)
	}
	if got != barePath {
		t.Fatalf("expected %q, got %q", barePath, got)
	}
}

func TestResolveLocalImagePathAbsoluteWithoutDoubling(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "Sys_Image")
	nestedDir := filepath.Join(base, "Fotos", "Productos")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	imagePath := filepath.Join(nestedDir, "0BONQ2.jpg")
	if err := os.WriteFile(imagePath, []byte("jpeg"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	got, err := resolveLocalImagePath(base, imagePath)
	if err != nil {
		t.Fatalf("resolve absolute path: %v", err)
	}
	if got != imagePath {
		t.Fatalf("expected %q, got %q", imagePath, got)
	}

	forwardSlash := strings.ReplaceAll(imagePath, `\`, "/")
	got, err = resolveLocalImagePath(base, forwardSlash)
	if err != nil {
		t.Fatalf("resolve absolute forward slash path: %v", err)
	}
	if got != imagePath {
		t.Fatalf("expected %q, got %q", imagePath, got)
	}

	missingAbsolute := filepath.Join(base, "Fotos", "Productos", "missing.jpg")
	_, err = resolveLocalImagePath(base, missingAbsolute)
	if err == nil {
		t.Fatal("expected missing absolute path error")
	}
	if strings.Contains(err.Error(), base+string(filepath.Separator)+base) {
		t.Fatalf("absolute missing path must not double base, got %v", err)
	}
}

func TestIsRemoteImageURL(t *testing.T) {
	t.Parallel()

	if !isRemoteImageURL("https://example.supabase.co/x.jpg") {
		t.Fatal("expected remote url true")
	}
	if isRemoteImageURL(`C:\tmp\a.jpg`) {
		t.Fatal("expected local path false")
	}
}
