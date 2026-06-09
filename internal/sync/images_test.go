package sync

import (
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
		{"relative/path.jpg", false},
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

func TestIsRemoteImageURL(t *testing.T) {
	t.Parallel()

	if !isRemoteImageURL("https://example.supabase.co/x.jpg") {
		t.Fatal("expected remote url true")
	}
	if isRemoteImageURL(`C:\tmp\a.jpg`) {
		t.Fatal("expected local path false")
	}
}
