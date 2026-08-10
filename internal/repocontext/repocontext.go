package repocontext

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/model"
)

const maxFiles = 80
const maxBytesPerFile = 6000
const maxTotalBytes = 80000

type RepoSnapshot struct {
	Name     string
	Path     string
	Language string
	Files    []FileSnippet
	Tree     []string
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
		files, err := gitx.ListTrackedFiles(repo.Path, 400)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", repo.Name, err)
		}
		// Prioritize hinted paths
		prio := map[string]int{}
		for _, h := range hints {
			if h.RepoName != "" && h.RepoName != repo.Name {
				continue
			}
			prio[filepath.Clean(h.FilePath)] = 100
		}
		scored := scoreFiles(files, prio)
		for _, f := range scored {
			if len(snap.Files) >= maxFiles || total >= maxTotalBytes {
				break
			}
			content, err := gitx.ReadFile(repo.Path, f)
			if err != nil {
				continue
			}
			trunc := false
			if len(content) > maxBytesPerFile {
				content = content[:maxBytesPerFile]
				trunc = true
			}
			total += len(content)
			snap.Files = append(snap.Files, FileSnippet{Path: f, Content: content, Trunc: trunc})
		}
		for i, f := range files {
			if i >= 120 {
				break
			}
			snap.Tree = append(snap.Tree, f)
		}
		out = append(out, snap)
	}
	return out, nil
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
	// simple insertion sort by score desc
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
