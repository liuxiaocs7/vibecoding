package executor

import (
	"os/exec"
	"strings"

	"github.com/ymhhh/vibecoding/internal/model"
)

// ProbeResult describes availability of one coding backend.
type ProbeResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // llm | agent
	Preset    string `json:"preset,omitempty"`
	Available bool   `json:"available"`
	Binary    string `json:"binary,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

// ProbeOptions controls how Probe walks the PATH and LLM key state.
type ProbeOptions struct {
	LookPath      func(string) (string, error)
	LLMConfigured bool
}

// Probe returns status for built-in LLM and known CLI presets.
func Probe(opts ProbeOptions) []ProbeResult {
	look := opts.LookPath
	if look == nil {
		look = exec.LookPath
	}
	out := []ProbeResult{
		{
			ID:        "llm",
			Name:      "Built-in LLM (VibeBot)",
			Type:      "llm",
			Available: opts.LLMConfigured,
			Hint:      llmHint(opts.LLMConfigured),
		},
	}
	out = append(out, probeCLI(look, "claude", "claude", "Claude Code", "Install Claude Code CLI and run `claude` to log in."))
	out = append(out, probeCursor(look))
	out = append(out, probeCLI(look, "codex", "codex", "OpenAI Codex CLI", "Install Codex CLI (`codex`) and authenticate."))
	out = append(out, ProbeResult{
		ID:        "custom",
		Name:      "Custom command",
		Type:      "agent",
		Preset:    "custom",
		Available: true,
		Hint:      "Provide command + args with {prompt} or enable promptStdin.",
	})
	return out
}

func llmHint(ok bool) string {
	if ok {
		return ""
	}
	return "Configure an OpenAI-compatible API key in Settings."
}

func probeCLI(look func(string) (string, error), id, bin, name, hint string) ProbeResult {
	path, err := look(bin)
	r := ProbeResult{
		ID:     id,
		Name:   name,
		Type:   "agent",
		Preset: id,
		Hint:   hint,
	}
	if err == nil && path != "" {
		r.Available = true
		r.Binary = path
		r.Hint = ""
	}
	return r
}

func probeCursor(look func(string) (string, error)) ProbeResult {
	r := ProbeResult{
		ID:     "cursor",
		Name:   "Cursor Agent",
		Type:   "agent",
		Preset: "cursor",
		Hint:   "Install Cursor CLI (`cursor-agent` or `agent`) and authenticate.",
	}
	for _, bin := range []string{"cursor-agent", "agent"} {
		if path, err := look(bin); err == nil && path != "" {
			r.Available = true
			r.Binary = path
			r.Hint = ""
			return r
		}
	}
	return r
}

// DisplayName returns a short label for UI / PRInfo.Executor.
func DisplayName(cfg model.ExecutorConfig) string {
	cfg = cfg.Normalize()
	if cfg.Type == "agent" {
		p := cfg.Preset
		if p == "" {
			p = "custom"
		}
		return "agent:" + p
	}
	return "llm"
}

// HasBinary reports whether a PATH entry exists (test helper / API).
func HasBinary(look func(string) (string, error), name string) bool {
	if look == nil {
		look = exec.LookPath
	}
	_, err := look(name)
	return err == nil
}

// MissingBinaries lists agent binaries that are not on PATH for the given config.
func MissingBinaries(cfg model.ExecutorConfig, look func(string) (string, error)) []string {
	cfg = cfg.Normalize()
	if cfg.Type != "agent" {
		return nil
	}
	if look == nil {
		look = exec.LookPath
	}
	switch cfg.Preset {
	case "claude":
		if !HasBinary(look, "claude") {
			return []string{"claude"}
		}
	case "cursor":
		if !HasBinary(look, "cursor-agent") && !HasBinary(look, "agent") {
			return []string{"cursor-agent", "agent"}
		}
	case "codex":
		if !HasBinary(look, "codex") {
			return []string{"codex"}
		}
	case "custom":
		cmd := strings.TrimSpace(cfg.Command)
		if cmd != "" && !strings.Contains(cmd, "/") && !HasBinary(look, cmd) {
			return []string{cmd}
		}
	}
	return nil
}
