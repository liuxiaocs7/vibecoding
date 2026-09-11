package api

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResumeUserPromptSkipsHintOnFirstTry(t *testing.T) {
	got := resumeUserPrompt(specRequestBody{Prompt: "分析开发方案"}, "")
	if got != "分析开发方案" {
		t.Fatalf("first try should be unchanged, got %q", got)
	}
	resumed := resumeUserPrompt(specRequestBody{Prompt: "分析开发方案", Resume: true, ResumePartial: `{"title":"draft"}`}, "")
	if resumed == "分析开发方案" || !containsAll(resumed, "分析开发方案", "draft") {
		t.Fatalf("session retry missing draft: %q", resumed)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
