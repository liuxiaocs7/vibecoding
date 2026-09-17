package repocontext

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func TestCollectPrefersAgentsMDAndSpecHints(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	writeRepoFile(t, dir, "AGENTS.md", "# Agents\nAlways run make test before finishing.\n")
	writeRepoFile(t, dir, "README.md", "# demo\n")
	writeRepoFile(t, dir, "pkg/target.go", "package pkg\n\nfunc Target() {}\n")
	for i := 0; i < 40; i++ {
		writeRepoFile(t, dir, filepath.Join("noise", fmt.Sprintf("f%d.go", i)), "package noise\n")
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init")

	repos := []model.GitRepo{{ID: "1", Name: "demo", Path: dir, Language: "go"}}
	hints := []model.SpecFileChange{{FilePath: "pkg/target.go", RepoName: "demo", Action: "modify"}}
	snaps, err := Collect(repos, hints)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 {
		t.Fatalf("snaps=%d", len(snaps))
	}
	s := snaps[0]
	if !strings.Contains(s.Instructions, "make test") {
		t.Fatalf("instructions missing AGENTS.md: %q", s.Instructions)
	}
	if !strings.Contains(s.Instructions, "README.md") {
		t.Fatalf("instructions missing README: %q", s.Instructions)
	}
	foundTarget := false
	for _, f := range s.Files {
		if f.Path == "pkg/target.go" {
			foundTarget = true
		}
	}
	if !foundTarget {
		t.Fatalf("expected Spec hint file in Files: %+v", s.Files)
	}
	if len(s.Files) > 0 && s.Files[0].Path != "pkg/target.go" {
		t.Fatalf("Spec hint should be first, got %q", s.Files[0].Path)
	}
	if len(s.Files) > maxFiles {
		t.Fatalf("too many files: %d", len(s.Files))
	}

	prompt := FormatForPrompt(snaps)
	if !strings.Contains(prompt, "Repository instructions") || !strings.Contains(prompt, "make test") {
		t.Fatalf("FormatForPrompt missing instructions:\n%s", prompt)
	}
	instr := InstructionsForPrompt(snaps)
	if !strings.Contains(instr, "make test") {
		t.Fatalf("InstructionsForPrompt=%q", instr)
	}
}
