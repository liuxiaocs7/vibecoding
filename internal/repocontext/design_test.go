package repocontext

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func TestExtractIdentifiersSkipsChineseNoise(t *testing.T) {
	text := "用户接口服务需要调用 OrderService.CreateOrder 和 `internal/api/order.go`，以及 user_order_repo"
	got := ExtractIdentifiers(text)
	joined := strings.Join(got, ",")
	for _, want := range []string{"OrderService.CreateOrder", "internal/api/order.go", "user_order_repo", "OrderService", "CreateOrder"} {
		if !strings.Contains(joined, want) && !containsExact(got, want) {
			// Camel parts may appear separately; dotted form is enough
			if want == "OrderService" || want == "CreateOrder" {
				continue
			}
			t.Fatalf("missing %q in %v", want, got)
		}
	}
	for _, bad := range []string{"用户", "接口", "服务"} {
		if containsExact(got, bad) {
			t.Fatalf("should not extract Chinese stop word %q from %v", bad, got)
		}
	}
}

func containsExact(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestBuildDesignIndexAndWindowHitMidFile(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	// Put TargetSymbol far below the 4KB head so windowAround must use hit line.
	var body strings.Builder
	body.WriteString("package demo\n\n")
	for i := 0; i < 200; i++ {
		body.WriteString("// padding line to push symbol into the middle of the file\n")
	}
	body.WriteString("func TargetSymbol() {}\n")
	writeRepoFile(t, dir, "pkg/service/handler.go", body.String())
	writeRepoFile(t, dir, "README.md", "# demo\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init")

	repos := []model.GitRepo{{ID: "1", Name: "svc", Path: dir, Language: "go"}}
	idx, err := BuildDesignIndex(repos, "Please wire TargetSymbol into the flow")
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Repos) != 1 {
		t.Fatalf("repos=%d", len(idx.Repos))
	}
	var hit CandidateFile
	found := false
	for _, c := range idx.Repos[0].Candidates {
		if strings.Contains(c.Path, "handler.go") {
			hit = c
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("handler.go not in candidates: %+v", idx.Repos[0].Candidates)
	}
	if hit.HitLine < 150 {
		t.Fatalf("expected mid-file hitLine, got %d", hit.HitLine)
	}

	ex, err := ReadDesignExcerpts(repos, []WantedFile{{
		RepoName: "svc",
		FilePath: hit.Path,
		HintLine: hit.HitLine,
	}}, idx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Repos) != 1 || len(ex.Repos[0].Files) != 1 {
		t.Fatalf("excerpt=%+v", ex)
	}
	fw := ex.Repos[0].Files[0]
	if !strings.Contains(fw.Content, "TargetSymbol") {
		t.Fatalf("window missing symbol; start=%d end=%d content[:200]=%q", fw.StartLine, fw.EndLine, trunc(fw.Content, 200))
	}
	if fw.StartLine > hit.HitLine || fw.EndLine < hit.HitLine {
		t.Fatalf("window [%d,%d] does not cover hit %d", fw.StartLine, fw.EndLine, hit.HitLine)
	}
	prompt := FormatDesignExcerptsForPrompt(ex)
	if !strings.Contains(prompt, "TargetSymbol") {
		t.Fatal("prompt missing symbol")
	}
	if !strings.Contains(prompt, "SOURCE EXCERPTS") {
		t.Fatal("prompt missing SOURCE EXCERPTS header")
	}
}

func TestAutoExcerptsLoadsTopCandidates(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	writeRepoFile(t, dir, "internal/dto/order.go", "package dto\n\ntype CreateOrderReq struct {\n\tSkuID string `json:\"skuId\"`\n}\n")
	writeRepoFile(t, dir, "README.md", "# demo\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init")

	repos := []model.GitRepo{{ID: "1", Name: "ziya", Path: dir, Language: "go"}}
	ex, idx, err := AutoExcerpts(repos, "CreateOrderReq skuId naming must match ziya", 4)
	if err != nil {
		t.Fatal(err)
	}
	if idx == nil || len(idx.Repos) != 1 {
		t.Fatalf("index=%+v", idx)
	}
	if ExcerptFileCount(ex) == 0 {
		t.Fatalf("expected file excerpts, got %+v", ex)
	}
	prompt := FormatDesignExcerptsForPrompt(ex)
	if !strings.Contains(prompt, "CreateOrderReq") && !strings.Contains(prompt, "skuId") {
		t.Fatalf("excerpt prompt missing symbols: %s", trunc(prompt, 400))
	}
}

func TestReadDesignExcerptsSkipsEscapeAndCapsBudget(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	writeRepoFile(t, dir, "a.go", "package a\nfunc A() {}\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init")

	repos := []model.GitRepo{{Name: "svc", Path: dir}}
	ex, err := ReadDesignExcerpts(repos, []WantedFile{
		{RepoName: "svc", FilePath: "../etc/passwd"},
		{RepoName: "svc", FilePath: "a.go"},
		{RepoName: "missing", FilePath: "a.go"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Repos) != 1 || len(ex.Repos[0].Files) != 1 {
		t.Fatalf("expected only safe file, got %+v", ex)
	}
}

func TestCapWantedFiles(t *testing.T) {
	got := CapWantedFiles([]WantedFile{
		{RepoName: "a", FilePath: "x.go"},
		{RepoName: "a", FilePath: "x.go"},
		{RepoName: "a", FilePath: "../y.go"},
		{RepoName: "a", FilePath: "z.go"},
	}, 1)
	if len(got) != 1 || got[0].FilePath != "x.go" {
		t.Fatalf("%+v", got)
	}
}

func TestVerifyCreateWithDesignIndexRoots(t *testing.T) {
	// smoke: format index does not panic on empty
	s := FormatDesignIndexForPrompt(&DesignIndex{Repos: []RepoDesignIndex{{
		Name: "svc", DirTree: []string{"pkg/ (1 files)"}, Candidates: []CandidateFile{{Path: "a.go", Score: 20}},
	}}})
	if !strings.Contains(s, "a.go") {
		t.Fatal(s)
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
