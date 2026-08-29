package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUniqueFilePath(t *testing.T) {
	dir := t.TempDir()
	p1 := uniqueFilePath(dir, "dev-spec.md")
	if p1 != filepath.Join(dir, "dev-spec.md") {
		t.Fatalf("first path=%s", p1)
	}
	if err := os.WriteFile(p1, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	p2 := uniqueFilePath(dir, "dev-spec.md")
	if p2 != filepath.Join(dir, "dev-spec-1.md") {
		t.Fatalf("second path=%s", p2)
	}
}

func TestSpecExportFileName(t *testing.T) {
	if got := specExportFileName(`foo/bar:baz`); got != "foo-bar-baz.md" {
		t.Fatalf("got %s", got)
	}
	if got := specExportFileName(""); got != "dev-spec.md" {
		t.Fatalf("empty got %s", got)
	}
}
