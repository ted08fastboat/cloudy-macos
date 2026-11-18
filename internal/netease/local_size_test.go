package netease

import (
	"os"
	"path/filepath"
	"testing"
)

// A local cover asked for at a size is marked with it, and Fetch still
// reads the file whole.
func TestSizedLocalCover(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover #1.jpg")
	if err := os.WriteFile(path, []byte("jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	sized := Sized(path, 96)
	if back, n := LocalSize(sized); back != path || n != 96 {
		t.Fatalf("LocalSize(%q) = %q, %d", sized, back, n)
	}
	if b, err := Fetch(sized); err != nil || string(b) != "jpeg" {
		t.Fatalf("Fetch = %q, %v", b, err)
	}
	if got := Sized("bar-cover", 96); got != "bar-cover" {
		t.Fatalf("a name that is no path became %q", got)
	}
	if _, n := LocalSize(Sized("https://p1.music.126.net/a.jpg", 96)); n != 0 {
		t.Fatal("a CDN cover is marked as local")
	}
}
