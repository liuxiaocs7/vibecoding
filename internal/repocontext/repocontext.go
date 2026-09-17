package repocontext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/model"
)

const maxFiles = 20
const maxBytesPerFile = 8000
const maxInstrBytes = 8000
const maxTotalBytes = 80000
const maxTreeEntries = 60

type RepoSnapshot struct {
	Name         string
	Path         string
	Language     string
	Instructions string // AGENTS.md / CLAUDE.md (+ README excerpt)
	Files        []FileSnippet
	Tree         []string
}

type FileSnippet struct {
	Path    string
	Content string
	Trunc   bool
}

func Collect(repos []model.GitRepo, hints []model.SpecFileChange) ([]RepoSnapshot, error) {
	var out []RepoSnapshot
	total := 0
	for _, repo := range repos {
		snap := RepoSnapshot{Name: repo.Name, Path: repo.Path, Language: repo.Language}
		snap.Instructions = loadRepoInstructions(repo.Path)
		total += len(snap.Instructions)

		files, err := gitx.ListTrackedFiles(repo.Path, 400)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", repo.Name, err)
		}

		seen := map[string]bool{}
		// 1) Spec-hinted paths first (exact reads).
		for _, h := range hints {
			if h.RepoName != "" && h.RepoName != repo.Name {
				continue
			}
			rel := filepath.Clean(strings.TrimSpace(h.FilePath))
			if rel == "" || rel == "." || seen[rel] {
				continue
			}
			if len(snap.Files) >= maxFiles || total >= maxTotalBytes {
				break
			}
			snip, n, ok := readSnippet(repo.Path, rel)
			if !ok {
				continue
			}
			seen[rel] = true
			total += n
			snap.Files = append(snap.Files, snip)
		}

		// 2) Light fill from scored tree (no blind dump of 80 sources).
		prio := map[string]int{}
		for _, h := range hints {
			if h.RepoName != "" && h.RepoName != repo.Name {
				continue
			}
			prio[filepath.Clean(h.FilePath)] = 100
		}
		for _, f := range scoreFiles(files, prio) {
			if seen[f] {
				continue
			}
			if len(snap.Files) >= maxFiles || total >= maxTotalBytes {
				break
			}
			// Skip instruction files already injected.
			base := strings.ToLower(filepath.Base(f))
			if base == "agents.md" || base == "claude.md" || base == "readme.md" {
				continue
			}
			snip, n, ok := readSnippet(repo.Path, f)
			if !ok {
				continue
			}
			seen[f] = true
			total += n
			snap.Files = append(snap.Files, snip)
		}

		for i, f := range files {
			if i >= maxTreeEntries {
				break
			}
			snap.Tree = append(snap.Tree, f)
		}
		out = append(out, snap)
	}
	return out, nil
}

func loadRepoInstructions(repoPath string) string {
	var parts []string
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		p := filepath.Join(repoPath, name)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(b))
		if text == "" {
			continue
		}
		if len(text) > maxInstrBytes {
			text = text[:maxInstrBytes] + "\n...[truncated]..."
		}
		parts = append(parts, fmt.Sprintf("----- %s -----\n%s", name, text))
		break // AGENTS.md preferred over CLAUDE.md
	}
	readme := filepath.Join(repoPath, "README.md")
	if b, err := os.ReadFile(readme); err == nil {
		text := strings.TrimSpace(string(b))
		if text != "" {
			if len(text) > maxInstrBytes {
				text = text[:maxInstrBytes] + "\n...[truncated]..."
			}
			parts = append(parts, "----- README.md -----\n"+text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func readSnippet(repoPath, rel string) (FileSnippet, int, bool) {
	content, err := gitx.ReadFile(repoPath, rel)
	if err != nil {
		return FileSnippet{}, 0, false
	}
	trunc := false
	if len(content) > maxBytesPerFile {
		content = content[:maxBytesPerFile]
		trunc = true
	}
	return FileSnippet{Path: rel, Content: content, Trunc: trunc}, len(content), true
}

func scoreFiles(files []string, prio map[string]int) []string {
	type item struct {
		path  string
		score int
	}
	var items []item
	for _, f := range files {
		s := 0
		if v, ok := prio[filepath.Clean(f)]; ok {
			s += v
		}
		lower := strings.ToLower(f)
		if strings.Contains(lower, "readme") {
			s += 5
		}
		if strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") ||
			strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".java") {
			s += 3
		}
		if strings.Contains(lower, "test") || strings.Contains(lower, "spec") {
			s += 1
		}
		if strings.Contains(lower, "node_modules") || strings.Contains(lower, "vendor/") {
			s -= 50
		}
		items = append(items, item{path: f, score: s})
	}
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && items[j].score > items[j-1].score {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.path
	}
	return out
}

func FormatForPrompt(snaps []RepoSnapshot) string {
	var b strings.Builder
	for _, s := range snaps {
		b.WriteString(fmt.Sprintf("### Repository: %s (%s)\nPath: %s\nLanguage: %s\n", s.Name, s.Path, s.Path, s.Language))
		if instr := strings.TrimSpace(s.Instructions); instr != "" {
			b.WriteString("\nRepository instructions:\n")
			b.WriteString(instr)
			b.WriteString("\n")
		}
		b.WriteString("File tree (sample):\n")
		for _, t := range s.Tree {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
		b.WriteString("\nSelected file contents:\n")
		for _, f := range s.Files {
			b.WriteString(fmt.Sprintf("\n----- BEGIN %s -----\n", f.Path))
			b.WriteString(f.Content)
			if f.Trunc {
				b.WriteString("\n...[truncated]...")
			}
			b.WriteString(fmt.Sprintf("\n----- END %s -----\n", f.Path))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// InstructionsForPrompt joins AGENTS/README blocks from snapshots for agent prompts.
func InstructionsForPrompt(snaps []RepoSnapshot) string {
	var parts []string
	for _, s := range snaps {
		if strings.TrimSpace(s.Instructions) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("Repository %s:\n%s", firstNonEmpty(s.Name, s.Path), s.Instructions))
	}
	return strings.Join(parts, "\n\n")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
