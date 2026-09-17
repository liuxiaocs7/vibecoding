package repocontext

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/model"
)

const (
	designMaxTreeDirs   = 120
	designMaxCandidates = 40
	designMaxWanted     = 12
	designMaxTotalBytes = 32000
	designWindowBefore  = 80
	designWindowAfter   = 80
	designMaxPerFile    = 8000
)

// DesignIndex is Pass-1 context: directory tree + candidate paths (no file bodies).
type DesignIndex struct {
	Repos []RepoDesignIndex
}

type RepoDesignIndex struct {
	Name       string
	Path       string
	Language   string
	DirTree    []string // compact directory listing
	Candidates []CandidateFile
}

type CandidateFile struct {
	Path    string
	Score   int
	HitLine int // 1-based; 0 if path-only match
	Reason  string
}

// DesignExcerpt is Pass-2 reading material around grep hits.
type DesignExcerpt struct {
	Repos []RepoExcerpt
}

type RepoExcerpt struct {
	Name  string
	Path  string
	Files []FileWindow
}

type FileWindow struct {
	Path      string
	StartLine int
	EndLine   int
	Content   string
	Trunc     bool
}

var (
	identCamel  = regexp.MustCompile(`\b[A-Z][a-zA-Z0-9]{2,}(?:[A-Z][a-zA-Z0-9]*)*\b`)
	identSnake  = regexp.MustCompile(`\b[a-z][a-z0-9]*(?:_[a-z0-9]+)+\b`)
	identPath   = regexp.MustCompile("`([^`]+)`")
	identDotted = regexp.MustCompile(`\b[a-zA-Z_][\w]*(?:\.[a-zA-Z_][\w]*)+\b`)
)

func skipPath(p string) bool {
	lower := strings.ToLower(p)
	return strings.Contains(lower, "node_modules/") ||
		strings.Contains(lower, "/vendor/") ||
		strings.HasPrefix(lower, "vendor/") ||
		strings.Contains(lower, "/.git/") ||
		strings.Contains(lower, "/dist/") ||
		strings.Contains(lower, "/build/")
}

// ExtractIdentifiers pulls CamelCase, snake_case, backtick paths, and dotted names
// from requirement text. Hot Chinese stop-words are ignored by design (ASCII idents only).
func ExtractIdentifiers(text string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || len(s) < 3 {
			return
		}
		if seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, m := range identPath.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range identDotted.FindAllString(text, -1) {
		add(m)
	}
	for _, m := range identCamel.FindAllString(text, -1) {
		add(m)
	}
	for _, m := range identSnake.FindAllString(text, -1) {
		add(m)
	}
	// Also pick path-like tokens with slashes.
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';' || r == '(' || r == ')' || r == '[' || r == ']'
	}) {
		if strings.Contains(tok, "/") && !strings.HasPrefix(tok, "http") {
			add(strings.Trim(tok, "`\"'"))
		}
	}
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

// BuildDesignIndex lists a compact directory tree and scores candidate files via
// identifier grep + path scoring. No file bodies are included.
func BuildDesignIndex(repos []model.GitRepo, queryText string) (*DesignIndex, error) {
	idents := ExtractIdentifiers(queryText)
	idx := &DesignIndex{}
	for _, repo := range repos {
		files, err := gitx.ListTrackedFiles(repo.Path, 2000)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", repo.Name, err)
		}
		var kept []string
		for _, f := range files {
			if skipPath(f) {
				continue
			}
			kept = append(kept, f)
		}
		ri := RepoDesignIndex{
			Name:     repo.Name,
			Path:     repo.Path,
			Language: repo.Language,
			DirTree:  compactDirTree(kept, designMaxTreeDirs),
		}

		score := map[string]int{}
		hitLine := map[string]int{}
		reason := map[string]string{}

		for _, f := range kept {
			base := strings.ToLower(filepath.Base(f))
			for _, id := range idents {
				idl := strings.ToLower(id)
				if strings.Contains(strings.ToLower(f), idl) || strings.Contains(base, idl) {
					score[f] += 20
					reason[f] = "path:" + id
				}
			}
			lower := strings.ToLower(f)
			if strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".ts") ||
				strings.HasSuffix(lower, ".tsx") || strings.HasSuffix(lower, ".js") ||
				strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".java") {
				score[f] += 2
			}
		}

		if len(idents) > 0 {
			hits, err := gitx.Grep(repo.Path, idents, 120)
			if err == nil {
				for _, h := range hits {
					if skipPath(h.Path) {
						continue
					}
					score[h.Path] += 40
					if hitLine[h.Path] == 0 || (h.Line > 0 && h.Line < hitLine[h.Path]) {
						hitLine[h.Path] = h.Line
					}
					reason[h.Path] = "grep"
				}
			}
		}

		type item struct {
			path  string
			score int
		}
		var items []item
		for p, s := range score {
			if s < 20 && hitLine[p] == 0 {
				continue // keep shortlist tight
			}
			items = append(items, item{p, s})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].score != items[j].score {
				return items[i].score > items[j].score
			}
			return items[i].path < items[j].path
		})
		limit := designMaxCandidates
		if len(items) < limit {
			limit = len(items)
		}
		for i := 0; i < limit; i++ {
			p := items[i].path
			ri.Candidates = append(ri.Candidates, CandidateFile{
				Path:    p,
				Score:   items[i].score,
				HitLine: hitLine[p],
				Reason:  reason[p],
			})
		}
		idx.Repos = append(idx.Repos, ri)
	}
	return idx, nil
}

func compactDirTree(files []string, maxDirs int) []string {
	dirs := map[string]int{}
	for _, f := range files {
		d := filepath.Dir(f)
		if d == "." {
			dirs["."]++
			continue
		}
		// Collapse to top 2 segments where possible.
		parts := strings.Split(d, string(filepath.Separator))
		key := d
		if len(parts) > 2 {
			key = filepath.Join(parts[0], parts[1]) + "/"
		} else {
			key = d + "/"
		}
		dirs[key]++
	}
	type kv struct {
		k string
		v int
	}
	var list []kv
	for k, v := range dirs {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].v != list[j].v {
			return list[i].v > list[j].v
		}
		return list[i].k < list[j].k
	})
	var out []string
	for i, it := range list {
		if i >= maxDirs {
			break
		}
		out = append(out, fmt.Sprintf("%s (%d files)", it.k, it.v))
	}
	return out
}

// WantedFile is a model-requested path for Pass 2.
type WantedFile struct {
	RepoName string
	FilePath string
	HintLine int
}

// ReadDesignExcerpts loads windows around hit lines for wanted files.
// Budget: ~24–32KB total, max designMaxWanted files.
func ReadDesignExcerpts(repos []model.GitRepo, wanted []WantedFile, index *DesignIndex) (*DesignExcerpt, error) {
	byName := map[string]model.GitRepo{}
	for _, r := range repos {
		byName[r.Name] = r
	}
	hitLines := map[string]int{} // repo|path -> line
	if index != nil {
		for _, ri := range index.Repos {
			for _, c := range ri.Candidates {
				if c.HitLine > 0 {
					hitLines[ri.Name+"|"+c.Path] = c.HitLine
				}
			}
		}
	}

	ex := &DesignExcerpt{}
	total := 0
	nFiles := 0
	repoIdx := map[string]int{}

	for _, w := range wanted {
		if nFiles >= designMaxWanted || total >= designMaxTotalBytes {
			break
		}
		repo, ok := byName[w.RepoName]
		if !ok {
			if len(repos) == 1 && strings.TrimSpace(w.RepoName) == "" {
				repo = repos[0]
			} else {
				continue
			}
		}
		rel := filepath.Clean(strings.TrimSpace(w.FilePath))
		if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		content, err := gitx.ReadFile(repo.Path, rel)
		if err != nil {
			continue
		}
		line := w.HintLine
		if line <= 0 {
			line = hitLines[repo.Name+"|"+rel]
		}
		win, start, end, trunc := windowAround(content, line, designWindowBefore, designWindowAfter, designMaxPerFile)
		if total+len(win) > designMaxTotalBytes {
			remain := designMaxTotalBytes - total
			if remain < 200 {
				break
			}
			win = win[:remain]
			trunc = true
		}
		fw := FileWindow{
			Path:      rel,
			StartLine: start,
			EndLine:   end,
			Content:   win,
			Trunc:     trunc,
		}
		if i, ok := repoIdx[repo.Name]; ok {
			ex.Repos[i].Files = append(ex.Repos[i].Files, fw)
		} else {
			repoIdx[repo.Name] = len(ex.Repos)
			ex.Repos = append(ex.Repos, RepoExcerpt{
				Name:  repo.Name,
				Path:  repo.Path,
				Files: []FileWindow{fw},
			})
		}
		total += len(win)
		nFiles++
	}
	return ex, nil
}

func windowAround(content string, hitLine, before, after, maxBytes int) (string, int, int, bool) {
	lines := strings.Split(content, "\n")
	n := len(lines)
	if n == 0 {
		return "", 1, 1, false
	}
	if hitLine <= 0 {
		// default: first maxBytes chars (still better than nothing)
		if len(content) <= maxBytes {
			return content, 1, n, false
		}
		return content[:maxBytes], 1, countLines(content[:maxBytes]), true
	}
	start := hitLine - before
	if start < 1 {
		start = 1
	}
	end := hitLine + after
	if end > n {
		end = n
	}
	chunk := strings.Join(lines[start-1:end], "\n")
	trunc := false
	if len(chunk) > maxBytes {
		chunk = chunk[:maxBytes]
		trunc = true
	}
	return chunk, start, end, trunc
}

func countLines(s string) int {
	if s == "" {
		return 1
	}
	return strings.Count(s, "\n") + 1
}

// FormatDesignIndexForPrompt renders Pass-1 context for the LLM.
func FormatDesignIndexForPrompt(idx *DesignIndex) string {
	if idx == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("Repository index (directories + candidate files; NO file bodies yet).\n")
	b.WriteString("Return wantedFiles as {repoName, filePath, hintLine?} only from this list or obvious creates.\n\n")
	for _, r := range idx.Repos {
		fmt.Fprintf(&b, "### Repository: %s\nPath: %s\nLanguage: %s\n", r.Name, r.Path, r.Language)
		b.WriteString("Directories:\n")
		for _, d := range r.DirTree {
			b.WriteString("- ")
			b.WriteString(d)
			b.WriteString("\n")
		}
		b.WriteString("Candidate files:\n")
		for _, c := range r.Candidates {
			fmt.Fprintf(&b, "- %s (score=%d", c.Path, c.Score)
			if c.HitLine > 0 {
				fmt.Fprintf(&b, ", hitLine=%d", c.HitLine)
			}
			if c.Reason != "" {
				fmt.Fprintf(&b, ", %s", c.Reason)
			}
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FormatDesignExcerptsForPrompt renders Pass-2 windows.
func FormatDesignExcerptsForPrompt(ex *DesignExcerpt) string {
	if ex == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("Source excerpts from associated local repositories (read-only scan).\n")
	b.WriteString("Only cite paths/symbols that appear below. Do not invent filenames.\n\n")
	for _, r := range ex.Repos {
		fmt.Fprintf(&b, "### Repository: %s (%s)\n", r.Name, r.Path)
		for _, f := range r.Files {
			fmt.Fprintf(&b, "\n----- BEGIN %s lines %d-%d -----\n", f.Path, f.StartLine, f.EndLine)
			b.WriteString(f.Content)
			if f.Trunc {
				b.WriteString("\n...[truncated]...")
			}
			fmt.Fprintf(&b, "\n----- END %s -----\n", f.Path)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// CapWantedFiles truncates and sanitizes model-requested files.
func CapWantedFiles(wanted []WantedFile, max int) []WantedFile {
	if max <= 0 {
		max = designMaxWanted
	}
	var out []WantedFile
	seen := map[string]bool{}
	for _, w := range wanted {
		rel := filepath.Clean(strings.TrimSpace(w.FilePath))
		if rel == "." || rel == "" || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			continue
		}
		key := w.RepoName + "|" + rel
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, WantedFile{RepoName: w.RepoName, FilePath: rel, HintLine: w.HintLine})
		if len(out) >= max {
			break
		}
	}
	return out
}
