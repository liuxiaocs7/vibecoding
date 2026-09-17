package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/llm"
	"github.com/ymhhh/vibecoding/internal/model"
	"github.com/ymhhh/vibecoding/internal/repocontext"
)

const llmSystem = `You are VibeBot, an autonomous coding agent.
Return ONLY a JSON object:
{
  "unifiedDiff": "<git unified diff patch>"
}
Rules:
- unifiedDiff must be a valid git unified diff (may include multiple files).
- Paths are relative to the worktree root (e.g. internal/foo.go). Never use absolute paths or .. segments.
- For new files use --- /dev/null and +++ b/path.
- For deletes use --- a/path and +++ /dev/null.
- Prefer small, reviewable hunks. Do NOT dump entire large files unless creating them.
- If extra instructions mention a sub-requirement, implement only that slice of work.
- Do not wrap JSON in markdown fences outside the JSON string value.
- Do not push remotes or merge default branches.`

// ChatClient is the LLM surface used by LLMExecutor (satisfied by *llm.Client).
type ChatClient interface {
	Chat(ctx context.Context, req llm.ChatRequest) (string, error)
}

// LLMExecutor generates unified diffs via the configured OpenAI-compatible model.
type LLMExecutor struct {
	Client ChatClient
}

func NewLLMExecutor(client ChatClient) *LLMExecutor {
	return &LLMExecutor{Client: client}
}

func (e *LLMExecutor) Name() string { return "llm" }

func (e *LLMExecutor) Run(ctx context.Context, req CodingRequest, emit Emit) (Result, error) {
	emit = ensureEmit(emit)
	if e == nil || e.Client == nil {
		return Result{}, fmt.Errorf("llm executor: missing client")
	}
	if strings.TrimSpace(req.RepoPath) == "" {
		return Result{}, fmt.Errorf("llm executor: empty RepoPath")
	}
	emit("coding", fmt.Sprintf("VibeBot generating code for %s...", firstNonEmpty(req.Title, req.RepoName)), "")

	text, err := e.generateRaw(ctx, req)
	if err != nil {
		return Result{}, err
	}

	diff, derr := ParseUnifiedDiffJSON(text)
	if derr == nil && strings.TrimSpace(diff) != "" {
		if err := gitx.ApplyUnifiedDiff(req.RepoPath, diff); err != nil {
			return Result{}, fmt.Errorf("apply unified diff: %w", err)
		}
		changes := ChangesFromUnifiedDiff(diff, req.RepoName)
		emit("coding", fmt.Sprintf("[%s] applied unified diff (%d file(s))", req.RepoName, len(changes)), "")
		if len(changes) == 0 {
			return Result{}, fmt.Errorf("LLM produced an empty unified diff")
		}
		return Result{Changes: changes}, nil
	}

	// Extreme fallback: model ignored the new prompt and returned full-file JSON.
	changes, ferr := ParseFileChangesJSON(text, req.Snapshots)
	if ferr != nil {
		return Result{}, fmt.Errorf("parse coding response: %v (also not unifiedDiff: %v)", ferr, derr)
	}
	n, err := gitx.ApplyFileWrites(req.RepoPath, changes, req.RepoName)
	if err != nil {
		return Result{}, fmt.Errorf("apply writes: %w", err)
	}
	emit("coding", fmt.Sprintf("[%s] applied %d file change(s) via legacy full-file fallback", req.RepoName, n), "")
	if n == 0 {
		return Result{}, fmt.Errorf("LLM produced no applicable file changes")
	}
	return Result{Changes: changes}, nil
}

func (e *LLMExecutor) generateRaw(ctx context.Context, req CodingRequest) (string, error) {
	specJSON, _ := json.Marshal(req.Spec)
	user := fmt.Sprintf("Issue: %s\nDescription: %s\n\n%s\nDevSpec JSON:\n%s\n\nRepository context:\n%s\n\nProduce the unifiedDiff JSON now.",
		req.Title, req.Description, req.Extra, string(specJSON), repocontext.FormatForPrompt(req.Snapshots))
	if resume := strings.TrimSpace(req.Resume); resume != "" {
		user += "\n\nResume / prior session note:\n" + resume
	}

	text, err := e.Client.Chat(ctx, llm.ChatRequest{
		ModelConfig: req.ModelConfig,
		System:      llmSystem,
		Messages:    []llm.ChatMessage{{Role: "user", Content: user}},
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		text, err = e.Client.Chat(ctx, llm.ChatRequest{
			ModelConfig: req.ModelConfig,
			System:      llmSystem,
			Messages:    []llm.ChatMessage{{Role: "user", Content: user}},
			Temperature: 0.2,
		})
		if err != nil {
			return "", err
		}
	}
	return text, nil
}

// ParseUnifiedDiffJSON extracts unifiedDiff from model JSON (or a raw diff body).
func ParseUnifiedDiffJSON(text string) (string, error) {
	raw := unwrapJSONObject(text)
	if raw == "" {
		return "", fmt.Errorf("empty")
	}
	var parsed struct {
		UnifiedDiff string `json:"unifiedDiff"`
		Diff        string `json:"diff"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		diff := strings.TrimSpace(firstNonEmpty(parsed.UnifiedDiff, parsed.Diff))
		if diff != "" {
			return stripDiffFence(diff), nil
		}
	}
	// Bare diff (no JSON envelope).
	if looksLikeUnifiedDiff(text) {
		return stripDiffFence(text), nil
	}
	return "", fmt.Errorf("no unifiedDiff field")
}

func stripDiffFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```diff")
		s = strings.TrimPrefix(s, "```DIFF")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	return s
}

func looksLikeUnifiedDiff(s string) bool {
	s = stripDiffFence(s)
	return strings.Contains(s, "diff --git ") || strings.Contains(s, "\n@@ ") || strings.HasPrefix(s, "--- ")
}

// ChangesFromUnifiedDiff builds SpecFileChange metadata (no ModifiedCode) from a patch.
func ChangesFromUnifiedDiff(diff, repoName string) []model.SpecFileChange {
	diff = stripDiffFence(diff)
	var out []model.SpecFileChange
	var cur *model.SpecFileChange
	flush := func() {
		if cur == nil || cur.FilePath == "" {
			return
		}
		if cur.Action == "" {
			cur.Action = "modify"
		}
		out = append(out, *cur)
		cur = nil
	}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			path := pathFromDiffGit(line)
			cur = &model.SpecFileChange{FilePath: path, RepoName: repoName, Action: "modify", Summary: "patched via unified diff"}
		case cur != nil && strings.HasPrefix(line, "new file mode"):
			cur.Action = "create"
		case cur != nil && strings.HasPrefix(line, "deleted file mode"):
			cur.Action = "delete"
		case cur != nil && strings.HasPrefix(line, "+++ b/"):
			p := strings.TrimPrefix(line, "+++ b/")
			if p != "/dev/null" && p != "" {
				cur.FilePath = p
			}
		case cur != nil && strings.HasPrefix(line, "+++ /dev/null"):
			cur.Action = "delete"
		case cur != nil && strings.HasPrefix(line, "--- /dev/null"):
			cur.Action = "create"
		}
	}
	flush()
	return out
}

func pathFromDiffGit(line string) string {
	// diff --git a/foo b/foo
	parts := strings.Fields(line)
	if len(parts) >= 4 {
		b := parts[3]
		return strings.TrimPrefix(b, "b/")
	}
	return ""
}

// ParseFileChangesJSON unwraps markdown fences and parses fileChanges (legacy fallback).
func ParseFileChangesJSON(text string, snaps []repocontext.RepoSnapshot) ([]model.SpecFileChange, error) {
	raw := unwrapJSONObject(text)
	if raw == "" {
		return nil, fmt.Errorf("parse coding response: empty")
	}
	var parsed struct {
		FileChanges []model.SpecFileChange `json:"fileChanges"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse coding response: %w", err)
	}
	if len(snaps) == 1 {
		for i := range parsed.FileChanges {
			if parsed.FileChanges[i].RepoName == "" {
				parsed.FileChanges[i].RepoName = snaps[0].Name
			}
		}
	}
	return parsed.FileChanges, nil
}

func unwrapJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```JSON")
		raw = strings.TrimPrefix(raw, "```")
		if i := strings.LastIndex(raw, "```"); i >= 0 {
			raw = raw[:i]
		}
		raw = strings.TrimSpace(raw)
	}
	if !strings.HasPrefix(raw, "{") {
		if i := strings.Index(raw, "{"); i >= 0 {
			if j := strings.LastIndex(raw, "}"); j > i {
				raw = raw[i : j+1]
			}
		}
	}
	return raw
}
