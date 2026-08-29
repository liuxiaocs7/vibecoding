// Package executor runs Auto-Dev coding steps via built-in LLM or external CLIs.
package executor

import (
	"context"
	"fmt"

	"github.com/ymhhh/vibecoding/internal/llm"
	"github.com/ymhhh/vibecoding/internal/model"
	"github.com/ymhhh/vibecoding/internal/repocontext"
)

// CodingRequest is the input for one coding pass in a single worktree.
type CodingRequest struct {
	RepoPath    string
	RepoName    string
	Title       string
	Description string
	Spec        *model.DevSpec
	Extra       string
	Snapshots   []repocontext.RepoSnapshot // LLM only
	Resume      string                     // agent session / heal
	ModelConfig model.ModelConfig          // LLM only
}

// Result is the output of one coding pass.
type Result struct {
	Changes   []model.SpecFileChange // LLM backfill; agent may be empty
	SessionID string
}

// Emit streams progress into Auto-Dev logs / SSE.
type Emit func(phase, msg, details string)

// Executor produces code changes for a worktree.
type Executor interface {
	Name() string
	Run(ctx context.Context, req CodingRequest, emit Emit) (Result, error)
}

// Build returns an Executor from global config.
func Build(cfg model.ExecutorConfig, client ChatClient) (Executor, error) {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	switch cfg.Type {
	case "agent":
		return NewAgentExecutor(cfg), nil
	default:
		if client == nil {
			return nil, fmt.Errorf("llm executor requires an LLM client")
		}
		return NewLLMExecutor(client), nil
	}
}

// Ensure *llm.Client satisfies ChatClient at compile time.
var _ ChatClient = (*llm.Client)(nil)

func noopEmit(phase, msg, details string) {}

func ensureEmit(emit Emit) Emit {
	if emit == nil {
		return noopEmit
	}
	return emit
}
