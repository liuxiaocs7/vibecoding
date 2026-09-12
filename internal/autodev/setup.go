package autodev

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ymhhh/vibecoding/internal/model"
)

func hasSetupCommand(sessions []worktreeSession) bool {
	for _, s := range sessions {
		if strings.TrimSpace(s.Repo.SetupCommand) != "" {
			return true
		}
	}
	return false
}

func (r *Runner) runSetupCommands(ctx context.Context, job *model.AutoDevJob, sessions []worktreeSession) error {
	for _, s := range sessions {
		cmdline := strings.TrimSpace(s.Repo.SetupCommand)
		if cmdline == "" {
			continue
		}
		_ = r.appendLog(job, "setup", fmt.Sprintf("[%s] running setup: %s", s.Repo.Name, cmdline), "")
		out, err := runSetupCommand(ctx, s.WTPath, cmdline)
		if err != nil {
			_ = r.appendLog(job, "setup", fmt.Sprintf("[%s] setup failed", s.Repo.Name), truncate(out, 2000))
			return fmt.Errorf("setup %s: %w\n%s", s.Repo.Name, err, truncate(out, 1500))
		}
		_ = r.appendLog(job, "setup", fmt.Sprintf("[%s] setup ok", s.Repo.Name), truncate(out, 800))
	}
	return nil
}

func runSetupCommand(ctx context.Context, dir, cmdline string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", cmdline)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdline)
	}
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
