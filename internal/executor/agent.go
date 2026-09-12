package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ymhhh/vibecoding/internal/model"
)

// CommandRunner starts a process; injectable for tests.
type CommandRunner func(ctx context.Context, name string, args []string, dir string, stdin io.Reader) *exec.Cmd

// AgentExecutor runs an external coding CLI inside the worktree.
type AgentExecutor struct {
	Cfg      model.ExecutorConfig
	LookPath func(string) (string, error)
	Runner   CommandRunner
}

func NewAgentExecutor(cfg model.ExecutorConfig) *AgentExecutor {
	return &AgentExecutor{Cfg: cfg.Normalize()}
}

func (e *AgentExecutor) Name() string {
	cfg := e.Cfg.Normalize()
	if cfg.Preset != "" {
		return "agent:" + cfg.Preset
	}
	return "agent"
}

func (e *AgentExecutor) Run(ctx context.Context, req CodingRequest, emit Emit) (Result, error) {
	emit = ensureEmit(emit)
	if strings.TrimSpace(req.RepoPath) == "" {
		return Result{}, fmt.Errorf("agent executor: empty RepoPath")
	}
	resolved, err := ResolvePreset(e.Cfg, e.LookPath)
	if err != nil {
		return Result{}, err
	}
	prompt := BuildAgentPrompt(req)
	args, usedPH := ExpandArgs(resolved.Args, prompt, req.Resume)
	useStdin := resolved.PromptStdin || (!usedPH && e.Cfg.PromptStdin)
	if !usedPH && !useStdin {
		// Presets always include {prompt}; custom may rely on stdin only.
		if e.Cfg.Preset == "custom" && e.Cfg.PromptStdin {
			useStdin = true
		} else if !usedPH {
			return Result{}, fmt.Errorf("agent executor: prompt not passed (need {prompt} or promptStdin)")
		}
	}

	timeout := time.Duration(e.Cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 1800 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdin io.Reader
	if useStdin {
		stdin = strings.NewReader(prompt)
	}

	emit("agent", fmt.Sprintf("Starting %s in %s", resolved.DisplayName, req.RepoName), resolved.Command+" "+strings.Join(args, " "))

	cmd := e.startCmd(runCtx, resolved.Command, args, req.RepoPath, stdin)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, err
	}
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start %s: %w", resolved.Command, err)
	}

	var stderrBuf bytes.Buffer
	doneOut := make(chan struct{})
	doneErr := make(chan struct{})
	sessionCh := make(chan string, 1)
	go func() {
		defer close(doneOut)
		sessionCh <- scanAgentOutput(stdout, emit, resolved.DisplayName)
	}()
	go func() {
		defer close(doneErr)
		sc := bufio.NewScanner(io.TeeReader(stderr, &stderrBuf))
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) != "" {
				emit("agent", line, "")
			}
		}
	}()

	waitErr := cmd.Wait()
	<-doneOut
	<-doneErr
	sessionID := <-sessionCh

	if runCtx.Err() == context.DeadlineExceeded {
		return Result{}, fmt.Errorf("agent %s timed out after %s", resolved.DisplayName, timeout)
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderrBuf.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return Result{}, fmt.Errorf("agent %s failed: %s", resolved.DisplayName, truncate(msg, 2000))
	}
	emit("agent", fmt.Sprintf("%s finished", resolved.DisplayName), "")
	return Result{SessionID: sessionID}, nil
}

func (e *AgentExecutor) startCmd(ctx context.Context, name string, args []string, dir string, stdin io.Reader) *exec.Cmd {
	runner := e.Runner
	if runner == nil {
		runner = defaultRunner
	}
	return runner(ctx, name, args, dir, stdin)
}

func defaultRunner(ctx context.Context, name string, args []string, dir string, stdin io.Reader) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ() // do not inject app OpenAPI keys
	if stdin != nil {
		cmd.Stdin = stdin
	}
	return cmd
}

func scanAgentOutput(r io.Reader, emit Emit, preset string) string {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sessionID := ""
	for sc.Scan() {
		line := sc.Text()
		if sid := sessionIDFromJSONLine(line); sid != "" {
			sessionID = sid
		}
		if msg := summarizeStreamJSON(line); msg != "" {
			emit("agent", msg, "")
			continue
		}
		if strings.TrimSpace(line) != "" {
			emit("agent", truncate(line, 500), "")
		}
	}
	return sessionID
}

func sessionIDFromJSONLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || line[0] != '{' {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return ""
	}
	for _, k := range []string{"session_id", "sessionId"} {
		if s, ok := obj[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// summarizeStreamJSON extracts a short tool/message line from Claude/Cursor stream-json.
func summarizeStreamJSON(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || line[0] != '{' {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return ""
	}
	if t, _ := obj["type"].(string); t != "" {
		switch t {
		case "assistant", "result", "system":
			if s := extractText(obj); s != "" {
				return truncate(s, 400)
			}
			return t
		case "tool_use", "tool_call", "tool_result":
			name, _ := obj["name"].(string)
			if name == "" {
				if tool, ok := obj["tool"].(map[string]any); ok {
					name, _ = tool["name"].(string)
				}
			}
			if name != "" {
				return "tool: " + name
			}
			return t
		}
	}
	if name, _ := obj["name"].(string); name != "" {
		return "tool: " + name
	}
	return ""
}

func extractText(obj map[string]any) string {
	if s, ok := obj["message"].(string); ok && s != "" {
		return s
	}
	if s, ok := obj["result"].(string); ok && s != "" {
		return s
	}
	if content, ok := obj["message"].(map[string]any); ok {
		if s, ok := content["content"].(string); ok {
			return s
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
