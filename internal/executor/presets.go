package executor

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/ymhhh/vibecoding/internal/model"
)

const promptPlaceholder = "{prompt}"

// ResolvedCmd is the concrete CLI invocation for an agent preset.
type ResolvedCmd struct {
	Command     string
	Args        []string
	PromptStdin bool
	DisplayName string
}

// ResolvePreset expands built-in or custom agent command templates.
// lookPath defaults to exec.LookPath when nil (injectable for tests).
func ResolvePreset(cfg model.ExecutorConfig, lookPath func(string) (string, error)) (ResolvedCmd, error) {
	cfg = cfg.Normalize()
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	switch cfg.Preset {
	case "claude":
		return ResolvedCmd{
			Command:     "claude",
			Args:        []string{"-p", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions", promptPlaceholder},
			DisplayName: "claude",
		}, nil
	case "cursor":
		bin := "cursor-agent"
		if _, err := lookPath(bin); err != nil {
			if _, err2 := lookPath("agent"); err2 == nil {
				bin = "agent"
			}
		}
		return ResolvedCmd{
			Command:     bin,
			Args:        []string{"--print", "--output-format", "stream-json", "--force", promptPlaceholder},
			DisplayName: "cursor",
		}, nil
	case "codex":
		return ResolvedCmd{
			Command:     "codex",
			Args:        []string{"exec", promptPlaceholder},
			DisplayName: "codex",
		}, nil
	case "custom":
		if strings.TrimSpace(cfg.Command) == "" {
			return ResolvedCmd{}, fmt.Errorf("custom preset requires command")
		}
		return ResolvedCmd{
			Command:     cfg.Command,
			Args:        append([]string(nil), cfg.Args...),
			PromptStdin: cfg.PromptStdin,
			DisplayName: "custom",
		}, nil
	default:
		return ResolvedCmd{}, fmt.Errorf("unknown preset %q", cfg.Preset)
	}
}

// ExpandArgs replaces {prompt} tokens; returns whether any arg still needs stdin.
func ExpandArgs(args []string, prompt string) (expanded []string, usedPlaceholder bool) {
	expanded = make([]string, 0, len(args))
	for _, a := range args {
		if strings.Contains(a, promptPlaceholder) {
			usedPlaceholder = true
			expanded = append(expanded, strings.ReplaceAll(a, promptPlaceholder, prompt))
			continue
		}
		expanded = append(expanded, a)
	}
	return expanded, usedPlaceholder
}
