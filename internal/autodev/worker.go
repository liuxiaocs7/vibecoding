package autodev

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ymhhh/go-common/logger"
	"github.com/ymhhh/vibecoding/internal/db"
	"github.com/ymhhh/vibecoding/internal/executor"
	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/llm"
	"github.com/ymhhh/vibecoding/internal/model"
	"github.com/ymhhh/vibecoding/internal/repocontext"
)

type Hub struct {
	mu        sync.Mutex
	subs      map[string]map[chan model.JobEvent]struct{}
	cancels   map[string]context.CancelFunc
	repoLocks sync.Map // path -> *sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		subs:    make(map[string]map[chan model.JobEvent]struct{}),
		cancels: make(map[string]context.CancelFunc),
	}
}

func (h *Hub) Subscribe(jobID string) (<-chan model.JobEvent, func()) {
	ch := make(chan model.JobEvent, 64)
	h.mu.Lock()
	if h.subs[jobID] == nil {
		h.subs[jobID] = make(map[chan model.JobEvent]struct{})
	}
	h.subs[jobID][ch] = struct{}{}
	h.mu.Unlock()
	unsub := func() {
		h.mu.Lock()
		delete(h.subs[jobID], ch)
		h.mu.Unlock()
		close(ch)
	}
	return ch, unsub
}

func (h *Hub) Publish(jobID string, ev model.JobEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[jobID] {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (h *Hub) Cancel(jobID string) bool {
	h.mu.Lock()
	cancel, ok := h.cancels[jobID]
	h.mu.Unlock()
	if ok && cancel != nil {
		cancel()
		return true
	}
	return false
}

func (h *Hub) lockRepo(path string) func() {
	v, _ := h.repoLocks.LoadOrStore(path, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

type Runner struct {
	Store        *db.Store
	LLM          *llm.Client
	Hub          *Hub
	WorktreeRoot string // e.g. ~/.vibecoding/worktrees
	// NewExecutor builds the coding executor; nil uses executor.Build with Store config + LLM.
	NewExecutor func(cfg model.ExecutorConfig) (executor.Executor, error)
}

func (r *Runner) Start(jobID string) {
	ctx, cancel := context.WithCancel(context.Background())
	r.Hub.mu.Lock()
	r.Hub.cancels[jobID] = cancel
	r.Hub.mu.Unlock()

	logger.L().WithField("job_id", jobID).Info("autodev started")

	go func() {
		defer func() {
			r.Hub.mu.Lock()
			delete(r.Hub.cancels, jobID)
			r.Hub.mu.Unlock()
			cancel()
		}()
		if err := r.run(ctx, jobID); err != nil {
			job, _ := r.Store.GetJob(jobID)
			if job == nil {
				return
			}
			if errors.Is(err, context.Canceled) || job.Status == model.JobCancelled {
				job.Status = model.JobCancelled
				job.Error = "cancelled"
				job.Phase = "cancelled"
				_ = r.Store.UpdateJob(job)
				_ = r.resetIssueToBacklog(job.IssueID)
				r.Hub.Publish(jobID, model.JobEvent{Type: "status", Status: string(model.JobCancelled), Message: "cancelled"})
				logger.L().WithField("job_id", jobID).Info("autodev cancelled")
				return
			}
			job.Status = model.JobFailed
			job.Error = err.Error()
			job.Phase = "failed"
			_ = r.Store.UpdateJob(job)
			_ = r.appendLog(job, "failed", err.Error(), "")
			_ = r.resetIssueToBacklog(job.IssueID)
			r.Hub.Publish(jobID, model.JobEvent{Type: "error", Phase: "failed", Error: err.Error(), Status: string(model.JobFailed)})
			logger.L().WithFields(logger.Fields{
				"job_id":   jobID,
				"issue_id": job.IssueID,
			}).WithError(err).Error("autodev failed")
		}
	}()
}

// resetIssueToBacklog moves a stuck in_progress issue back after fail/cancel.
func (r *Runner) resetIssueToBacklog(issueID string) error {
	issue, err := r.Store.GetIssue(issueID)
	if err != nil || issue == nil {
		return err
	}
	if issue.Status != model.StatusInProgress {
		return nil
	}
	issue.Status = model.StatusBacklog
	issue.UpdatedAt = model.NowISO()
	return r.Store.UpsertIssue(*issue)
}

func (r *Runner) run(ctx context.Context, jobID string) error {
	job, err := r.Store.GetJob(jobID)
	if err != nil || job == nil {
		return fmt.Errorf("job not found")
	}
	job.Status = model.JobRunning
	job.Phase = "analyzing"
	_ = r.Store.UpdateJob(job)

	issue, err := r.Store.GetIssue(job.IssueID)
	if err != nil || issue == nil {
		return fmt.Errorf("issue not found")
	}
	proj, err := r.Store.GetProject(issue.ProjectID)
	if err != nil || proj == nil {
		return fmt.Errorf("project not found")
	}
	if !issue.SpecReadyForDev() {
		return fmt.Errorf("issue has no Dev Spec")
	}

	var repos []model.GitRepo
	for _, rid := range issue.AssociatedRepoIDs {
		for _, gr := range proj.GitRepos {
			if gr.ID == rid {
				repos = append(repos, gr)
			}
		}
	}
	if len(repos) == 0 {
		return fmt.Errorf("no associated repositories")
	}
	for _, repo := range repos {
		if _, err := gitx.ValidateRepo(repo.Path); err != nil {
			return fmt.Errorf("repo %s: %w", repo.Name, err)
		}
	}

	job.Status = model.JobRunning
	_ = r.Store.UpdateJob(job)

	cfg, _ := r.Store.GetModelConfig()
	if proj.UseCustomModelConfig && proj.CustomModelConfig != nil && strings.TrimSpace(proj.CustomModelConfig.OpenAIAPIKey) != "" {
		cfg = *proj.CustomModelConfig
	}

	execCfg, _ := r.Store.GetExecutorConfig()
	execCfg = execCfg.Normalize()
	execName := executor.DisplayName(execCfg)
	job.Executor = execName
	_ = r.Store.UpdateJob(job)

	prefix := "ai-dev/"
	if proj.BranchPrefixConfig != nil && proj.BranchPrefixConfig.AutoDevPrefix != "" {
		prefix = proj.BranchPrefixConfig.AutoDevPrefix
	}
	shortID := issue.ID
	if len(shortID) > 8 {
		shortID = shortID[len(shortID)-8:]
	}
	branchName := fmt.Sprintf("%sissue-%s", prefix, shortID)

	progress := func(p int, phase, msg string) error {
		if ctx.Err() != nil {
			job.Status = model.JobCancelled
			job.Error = "cancelled"
			job.Phase = "cancelled"
			_ = r.Store.UpdateJob(job)
			_ = r.resetIssueToBacklog(issue.ID)
			r.Hub.Publish(jobID, model.JobEvent{Type: "status", Status: string(model.JobCancelled), Message: "cancelled"})
			return context.Canceled
		}
		job.Progress = p
		job.Phase = phase
		_ = r.Store.UpdateJob(job)
		_ = r.appendLog(job, phase, msg, "")
		issue.AutoDevProgress = p
		issue.Status = model.StatusInProgress
		_ = r.Store.UpsertIssue(*issue)
		r.Hub.Publish(jobID, model.JobEvent{Type: "progress", Phase: phase, Message: msg, Progress: p, Status: string(model.JobRunning)})
		logger.L().WithFields(logger.Fields{
			"job_id":   jobID,
			"issue_id": issue.ID,
			"phase":    phase,
			"progress": p,
		}).Info(msg)
		return nil
	}

	if err := progress(10, "analyzing", "Analyzing Dev Spec and sampling repository context..."); err != nil {
		return err
	}

	if err := progress(25, "branching", fmt.Sprintf("Preparing isolated worktrees for %s", branchName)); err != nil {
		return err
	}
	sessions, baseBranch, err := r.prepareWorktrees(repos, issue.ID, branchName)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		_ = r.appendLog(job, "branching", fmt.Sprintf("[%s] worktree %s (base=%s, branch=%s)", s.Repo.Name, s.WTPath, s.Base, s.Branch), "")
	}
	wtRepos := sessionsToRepos(sessions)

	var quality *model.QualityGate
	if issue.HasSubRequirements() {
		quality, err = r.developSubs(ctx, job, issue, wtRepos, cfg, execCfg, progress)
		if err != nil {
			return err
		}
	} else {
		quality, err = r.developOne(ctx, job, issue, wtRepos, cfg, execCfg, issue.DevSpec, issue.Title, issue.Description, "", 40, 90, true, progress)
		if err != nil {
			return err
		}
	}

	pr := &model.PRInfo{
		ID:          "pr-" + uuid.NewString()[:8],
		BranchName:  branchName,
		Title:       "feat: " + issue.Title,
		Description: r.prDescription(*issue),
		Author:      "VibeBot",
		CreatedAt:   model.NowISO(),
		Status:      "open",
		DiffStats:   r.collectDiffStats(wtRepos),
		Worktrees:   sessionsToRefs(sessions),
		BaseBranch:  baseBranch,
		Quality:     quality,
		Executor:    execName,
	}
	job.Executor = execName
	job.Status = model.JobCompleted
	job.Progress = 100
	job.Phase = "completed"
	job.PRInfo = pr
	_ = r.Store.UpdateJob(job)

	fresh, _ := r.Store.GetIssue(issue.ID)
	if fresh != nil {
		issue = fresh
	}
	issue.Status = model.StatusInReview
	issue.AutoDevProgress = 100
	issue.PRInfo = pr
	issue.CurrentSubID = ""
	issue.ReworkSubID = ""
	issue.UpdatedAt = model.NowISO()
	_ = r.appendLog(job, "completed", "Code committed; issue moved to in_review", "")
	logs, _ := r.Store.ListJobLogs(job.ID)
	if len(logs) > 0 {
		issue.AutoDevLogs = append(issue.AutoDevLogs, logs...)
	}
	if issue.HasSubRequirements() && issue.DevSpec != nil {
		issue.DevSpec.FileChanges = issue.AggregatedFileChanges()
	}
	_ = r.Store.UpsertIssue(*issue)

	logger.L().WithFields(logger.Fields{
		"job_id":   jobID,
		"issue_id": issue.ID,
		"branch":   branchName,
	}).Info("autodev completed")

	r.Hub.Publish(jobID, model.JobEvent{
		Type:     "done",
		Phase:    "completed",
		Message:  "Auto-dev completed",
		Progress: 100,
		Status:   string(model.JobCompleted),
		PRInfo:   pr,
	})
	return nil
}

func (r *Runner) appendLog(job *model.AutoDevJob, phase, message, details string) error {
	log := model.AutoDevLog{
		ID:        "log-" + uuid.NewString()[:10],
		Timestamp: time.Now().Format("15:04:05"),
		Phase:     phase,
		Message:   message,
		Details:   details,
	}
	if err := r.Store.AppendJobLog(job.ID, job.IssueID, log); err != nil {
		return err
	}
	// Also persist onto issue for UI convenience
	iss, _ := r.Store.GetIssue(job.IssueID)
	if iss != nil {
		iss.AutoDevLogs = append(iss.AutoDevLogs, log)
		_ = r.Store.UpsertIssue(*iss)
	}
	r.Hub.Publish(job.ID, model.JobEvent{Type: "log", Phase: phase, Message: message, Details: details, Log: &log, Progress: job.Progress})
	return nil
}

func (r *Runner) developSubs(
	ctx context.Context,
	job *model.AutoDevJob,
	issue *model.Issue,
	repos []model.GitRepo,
	cfg model.ModelConfig,
	execCfg model.ExecutorConfig,
	progress func(int, string, string) error,
) (*model.QualityGate, error) {
	issue.NormalizeSubs()
	indices := make([]int, 0, len(issue.SubRequirements))
	reworkID := strings.TrimSpace(issue.ReworkSubID)
	if reworkID != "" {
		found := false
		for i, sub := range issue.SubRequirements {
			if sub.ID == reworkID {
				indices = append(indices, i)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("rework sub-requirement %s not found", reworkID)
		}
	} else {
		for i := range issue.SubRequirements {
			indices = append(indices, i)
		}
	}
	n := len(indices)
	if n == 0 {
		return nil, fmt.Errorf("no sub-requirements to develop")
	}

	var lastQuality *model.QualityGate
	for k, idx := range indices {
		sub := &issue.SubRequirements[idx]
		if sub.DevSpec == nil {
			return nil, fmt.Errorf("sub-requirement %q has no Dev Spec", sub.Title)
		}
		sub.Status = model.SubReqInProgress
		issue.CurrentSubID = sub.ID
		issue.UpdatedAt = model.NowISO()
		_ = r.Store.UpsertIssue(*issue)

		pStart := 20 + 70*k/n
		pEnd := 20 + 70*(k+1)/n
		label := fmt.Sprintf("[%d/%d] %s", k+1, n, sub.Title)
		if err := progress(pStart, "coding", "Developing sub-requirement "+label); err != nil {
			return nil, err
		}
		r.Hub.Publish(job.ID, model.JobEvent{
			Type:             "progress",
			Phase:            "coding",
			Message:          "Developing " + label,
			Progress:         pStart,
			Status:           string(model.JobRunning),
			SubRequirementID: sub.ID,
			SubTitle:         sub.Title,
			SubIndex:         k + 1,
			SubTotal:         n,
		})

		var extra strings.Builder
		extra.WriteString(fmt.Sprintf("This is sub-requirement %d of %d for parent issue %q.\n", sub.Order, len(issue.SubRequirements), issue.Title))
		extra.WriteString("Implement ONLY this sub-requirement. Do not undo earlier sub-requirement work.\n")
		if reworkID != "" {
			extra.WriteString("This is a REWORK pass: previous code is already on the branch. Update this sub's files.\n")
		}
		for _, sib := range issue.SubRequirements {
			if sib.ID == sub.ID {
				continue
			}
			extra.WriteString(fmt.Sprintf("- sibling [%d] %s (%s)\n", sib.Order, sib.Title, sib.Status))
		}

		q, err := r.developOne(ctx, job, issue, repos, cfg, execCfg, sub.DevSpec, sub.Title, firstNonEmpty(sub.Description, issue.Description), extra.String(), pStart, pEnd, k == n-1, progress)
		if err != nil {
			sub.Status = model.SubReqFailed
			issue.UpdatedAt = model.NowISO()
			_ = r.Store.UpsertIssue(*issue)
			return q, fmt.Errorf("sub-requirement %q: %w", sub.Title, err)
		}
		lastQuality = q

		sha := ""
		if len(repos) > 0 {
			sha, _ = gitx.HeadSHA(repos[0].Path)
		}
		if fresh, err := r.Store.GetIssue(issue.ID); err == nil && fresh != nil {
			*issue = *fresh
		}
		if target := issue.SubByID(sub.ID); target != nil {
			target.Status = model.SubReqDone
			target.CommitSHA = sha
		}
		issue.CurrentSubID = sub.ID
		issue.UpdatedAt = model.NowISO()
		_ = r.Store.UpsertIssue(*issue)
		_ = r.appendLog(job, "committing", fmt.Sprintf("Completed sub-requirement %s", label), sha)
	}

	issue.ReworkSubID = ""
	issue.CurrentSubID = ""
	issue.UpdatedAt = model.NowISO()
	_ = r.Store.UpsertIssue(*issue)
	return lastQuality, nil
}

func (r *Runner) buildExecutor(execCfg model.ExecutorConfig) (executor.Executor, error) {
	if r.NewExecutor != nil {
		return r.NewExecutor(execCfg)
	}
	return executor.Build(execCfg, r.LLM)
}

func (r *Runner) developOne(
	ctx context.Context,
	job *model.AutoDevJob,
	issue *model.Issue,
	repos []model.GitRepo,
	cfg model.ModelConfig,
	execCfg model.ExecutorConfig,
	spec *model.DevSpec,
	title, desc, extra string,
	pStart, pEnd int,
	withLint bool,
	progress func(int, string, string) error,
) (*model.QualityGate, error) {
	if spec == nil {
		return nil, fmt.Errorf("missing Dev Spec")
	}
	span := pEnd - pStart
	if span < 8 {
		span = 8
	}
	pCode := pStart
	pTest := pStart + span*50/100
	pLint := pStart + span*75/100
	pCommit := pEnd
	if pTest <= pCode {
		pTest = pCode + 1
	}

	ex, err := r.buildExecutor(execCfg)
	if err != nil {
		return nil, err
	}
	emit := func(phase, msg, details string) {
		_ = r.appendLog(job, phase, msg, details)
	}

	maxHeal := execCfg.Normalize().MaxHeal
	quality := &model.QualityGate{}
	healExtra := extra
	var sessionID string

	for round := 0; round <= maxHeal; round++ {
		if round > 0 {
			quality.RepairRounds = round
			if err := progress(pCode, "coding", fmt.Sprintf("Repair round %d/%d for %s...", round, maxHeal, title)); err != nil {
				return quality, err
			}
			_ = r.appendLog(job, "testing", fmt.Sprintf("tests failed, repair round %d/%d", round, maxHeal), "")
		} else if err := progress(pCode, "coding", fmt.Sprintf("Generating code for %s via %s...", title, ex.Name())); err != nil {
			return quality, err
		}

		var allChanges []model.SpecFileChange
		for _, repo := range repos {
			snaps, err := repocontext.Collect([]model.GitRepo{repo}, spec.FileChanges)
			if err != nil {
				return quality, err
			}
			req := executor.CodingRequest{
				RepoPath:    repo.Path,
				RepoName:    repo.Name,
				Title:       title,
				Description: desc,
				Spec:        spec,
				Extra:       healExtra,
				Snapshots:   snaps,
				Resume:      sessionID,
				ModelConfig: cfg,
			}
			res, err := ex.Run(ctx, req, emit)
			if err != nil {
				return quality, fmt.Errorf("executor %s on %s: %w", ex.Name(), repo.Name, err)
			}
			if res.SessionID != "" {
				sessionID = res.SessionID
			}
			allChanges = append(allChanges, res.Changes...)
		}

		if len(allChanges) > 0 {
			spec.FileChanges = allChanges
			spec.UpdatedAt = model.NowISO()
			if issue.HasSubRequirements() {
				if fresh, e := r.Store.GetIssue(issue.ID); e == nil && fresh != nil {
					*issue = *fresh
				}
				if target := issue.SubByID(issue.CurrentSubID); target != nil && target.DevSpec != nil {
					target.DevSpec.FileChanges = allChanges
					target.DevSpec.UpdatedAt = spec.UpdatedAt
				}
			} else if issue.DevSpec != nil {
				issue.DevSpec.FileChanges = allChanges
				issue.DevSpec.UpdatedAt = spec.UpdatedAt
			}
			_ = r.Store.UpsertIssue(*issue)
		}

		if err := progress(pTest, "testing", fmt.Sprintf("Running tests after %s...", title)); err != nil {
			return quality, err
		}
		testFailed := false
		var failBuf strings.Builder
		for _, repo := range repos {
			out, skipped, err := runTests(repo)
			if skipped {
				_ = r.appendLog(job, "testing", fmt.Sprintf("[%s] no test command detected — skipped", repo.Name), out)
				continue
			}
			quality.TestsRan = true
			if err != nil {
				testFailed = true
				quality.TestsPassed = false
				failBuf.WriteString(fmt.Sprintf("[%s]\n%s\n", repo.Name, out))
				_ = r.appendLog(job, "testing", fmt.Sprintf("[%s] tests failed", repo.Name), truncate(out, 1500))
				continue
			}
			_ = r.appendLog(job, "testing", fmt.Sprintf("[%s] tests passed", repo.Name), truncate(out, 800))
		}
		if !testFailed {
			if quality.TestsRan {
				quality.TestsPassed = true
			}
			quality.TestsOutput = ""
			break
		}
		quality.TestsOutput = truncate(failBuf.String(), 4000)
		if round == maxHeal {
			return quality, fmt.Errorf("tests failed after %d repair round(s):\n%s", maxHeal, truncate(quality.TestsOutput, 1500))
		}
		healExtra = extra
		if healExtra != "" {
			healExtra += "\n\n"
		}
		healExtra += "Previous tests failed. Fix the failures using this output:\n" + quality.TestsOutput
	}

	if withLint {
		if err := progress(pLint, "linting", "Running lightweight static checks..."); err != nil {
			return quality, err
		}
		var lintBuf strings.Builder
		lintFailed := false
		for _, repo := range repos {
			out, skipped, err := runLint(repo)
			if skipped {
				_ = r.appendLog(job, "linting", fmt.Sprintf("[%s] lint skipped", repo.Name), "")
				continue
			}
			quality.LintRan = true
			if err != nil {
				lintFailed = true
				lintBuf.WriteString(fmt.Sprintf("[%s]\n%s\n", repo.Name, out))
				_ = r.appendLog(job, "linting", fmt.Sprintf("[%s] lint warnings (non-blocking): %v", repo.Name, err), truncate(out, 600))
			} else {
				_ = r.appendLog(job, "linting", fmt.Sprintf("[%s] lint ok", repo.Name), truncate(out, 400))
			}
		}
		quality.LintPassed = quality.LintRan && !lintFailed
		quality.LintOutput = truncate(lintBuf.String(), 2000)
	}

	if err := progress(pCommit, "committing", fmt.Sprintf("Committing %s...", title)); err != nil {
		return quality, err
	}
	for _, repo := range repos {
		dirty, err := gitx.IsDirty(repo.Path)
		if err != nil {
			return quality, fmt.Errorf("dirty check %s: %w", repo.Name, err)
		}
		if !dirty {
			_ = r.appendLog(job, "committing", fmt.Sprintf("[%s] nothing to commit for %s", repo.Name, title), "")
			continue
		}
		msg := fmt.Sprintf("feat: %s\n\nAuto-generated by Vibecoding for issue %s", title, issue.ID)
		if extra != "" {
			msg = fmt.Sprintf("feat: %s\n\nSub-requirement of %s (%s).\nAuto-generated by Vibecoding.", title, issue.Title, issue.ID)
		}
		if err := gitx.CommitAll(repo.Path, msg); err != nil {
			return quality, fmt.Errorf("commit %s: %w", repo.Name, err)
		}
		_ = r.appendLog(job, "committing", fmt.Sprintf("[%s] committed %s", repo.Name, title), "")
	}
	return quality, nil
}

func (r *Runner) prDescription(issue model.Issue) string {
	if !issue.HasSubRequirements() {
		return "Auto-dev changes generated by Vibecoding (local branch review)."
	}
	var b strings.Builder
	b.WriteString("Sequential Auto-dev by Vibecoding.\n\nSub-requirements:\n")
	for _, sub := range issue.SubRequirements {
		sha := sub.CommitSHA
		if sha == "" {
			sha = string(sub.Status)
		}
		b.WriteString(fmt.Sprintf("- [%d] %s (%s)\n", sub.Order, sub.Title, sha))
	}
	return b.String()
}

func (r *Runner) collectDiffStats(repos []model.GitRepo) model.DiffStats {
	var stats model.DiffStats
	for _, repo := range repos {
		st, _ := gitx.DiffStats(repo.Path, repo.DefaultBranch)
		stats.Additions += st.Additions
		stats.Deletions += st.Deletions
		stats.FilesChanged += st.FilesChanged
	}
	return stats
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func runTests(repo model.GitRepo) (output string, skipped bool, err error) {
	switch {
	case fileExists(filepath.Join(repo.Path, "go.mod")):
		out, skipped, err := runCmd(repo.Path, "go", "test", "./...")
		if skipped {
			return out, true, nil
		}
		// Empty modules (go.mod only) report "no packages to test" with exit 1.
		if err != nil && strings.Contains(out, "no packages to test") {
			return out, true, nil
		}
		return out, false, err
	case fileExists(filepath.Join(repo.Path, "package.json")):
		// prefer npm test if script exists — just try
		if fileExists(filepath.Join(repo.Path, "pnpm-lock.yaml")) {
			return runCmd(repo.Path, "pnpm", "test", "--if-present")
		}
		return runCmd(repo.Path, "npm", "test", "--if-present")
	case fileExists(filepath.Join(repo.Path, "pyproject.toml")) || fileExists(filepath.Join(repo.Path, "pytest.ini")):
		return runCmd(repo.Path, "pytest", "-q")
	default:
		return "", true, nil
	}
}

func runLint(repo model.GitRepo) (output string, skipped bool, err error) {
	if fileExists(filepath.Join(repo.Path, "go.mod")) {
		return runCmd(repo.Path, "go", "vet", "./...")
	}
	if fileExists(filepath.Join(repo.Path, "tsconfig.json")) {
		return runCmd(repo.Path, "npx", "--no-install", "tsc", "--noEmit")
	}
	return "", true, nil
}

func runCmd(dir, name string, args ...string) (string, bool, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", true, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), false, err
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
