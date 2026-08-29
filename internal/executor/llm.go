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
  "fileChanges": [
    {
      "filePath": "relative/path.ext",
      "repoName": "exact-repo-name",
      "action": "create|modify|delete",
      "summary": "what changed",
      "modifiedCode": "FULL file contents for create/modify (required unless delete)"
    }
  ]
}
Rules:
- Use exact repoName values from context.
- Prefer modifying existing files listed in the Dev Spec.
- modifiedCode must be the complete file content.
- Do not wrap JSON in markdown.
- If extra instructions mention a sub-requirement, implement only that slice of work.
- Do not push remotes or merge default branches.`

// ChatClient is the LLM surface used by LLMExecutor (satisfied by *llm.Client).
type ChatClient interface {
	Chat(ctx context.Context, req llm.ChatRequest) (string, error)
}

// LLMExecutor generates full-file JSON patches via the configured OpenAI-compatible model.
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

	changes, err := e.generate(ctx, req)
	if err != nil {
		return Result{}, err
	}
	n, err := gitx.ApplyFileWrites(req.RepoPath, changes, req.RepoName)
	if err != nil {
		return Result{}, fmt.Errorf("apply writes: %w", err)
	}
	emit("coding", fmt.Sprintf("[%s] applied %d file change(s)", req.RepoName, n), "")
	if n == 0 {
		return Result{}, fmt.Errorf("LLM produced no applicable file changes")
	}
	return Result{Changes: changes}, nil
}

func (e *LLMExecutor) generate(ctx context.Context, req CodingRequest) ([]model.SpecFileChange, error) {
	specJSON, _ := json.Marshal(req.Spec)
	user := fmt.Sprintf("Issue: %s\nDescription: %s\n\n%s\nDevSpec JSON:\n%s\n\nRepository context:\n%s\n\nProduce the fileChanges JSON now.",
		req.Title, req.Description, req.Extra, string(specJSON), repocontext.FormatForPrompt(req.Snapshots))

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
			return nil, err
		}
	}
	return ParseFileChangesJSON(text, req.Snapshots)
}

// ParseFileChangesJSON unwraps markdown fences and parses fileChanges.
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
