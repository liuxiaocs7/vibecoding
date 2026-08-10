package autodev

import (
	"context"
	"encoding/json"
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
	"github.com/ymhhh/vibecoding/internal/gitx"
	"github.com/ymhhh/vibecoding/internal/llm"
	"github.com/ymhhh/vibecoding/internal/model"
	"github.com/ymhhh/vibecoding/internal/repocontext"
)

type Hub struct {
	mu    sync.Mutex
	subs  map[string]map[chan model.JobEvent]struct{}
	cancels map[string]context.CancelFunc
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
	Store *db.Store
	LLM   *llm.Client
	Hub   *Hub
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
			if job != nil && job.Status != model.JobCancelled {
				job.Status = model.JobFailed
				job.Error = err.Error()
				job.Phase = "failed"
				_ = r.Store.UpdateJob(job)
				_ = r.appendLog(job, "failed", err.Error(), "")
				r.Hub.Publish(jobID, model.JobEvent{Type: "error", Phase: "failed", Error: err.Error(), Status: string(model.JobFailed)})
				logger.L().WithFields(logger.Fields{
					"job_id":   jobID,
					"issue_id": job.IssueID,
				}).WithError(err).Error("autodev failed")
			} else if err == context.Canceled {
				logger.L().WithField("job_id", jobID).Info("autodev cancelled")
			}
		}
	}()
}

func (r *Runner) run(ctx context.Context, jobID string) error {
	job, err := r.Store.GetJob(jobID)
	if err != nil || job == nil {
		return fmt.Errorf("job not found")
	}
	issue, err := r.Store.GetIssue(job.IssueID)
	if err != nil || issue == nil {
		return fmt.Errorf("issue not found")
	}
	proj, err := r.Store.GetProject(issue.ProjectID)
	if err != nil || proj == nil {
		return fmt.Errorf("project not found")
	}
	if issue.DevSpec == nil {
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

	// Acquire locks for all repos
	var unlocks []func()
	for _, repo := range repos {
		unlocks = append(unlocks, r.Hub.lockRepo(repo.Path))
	}
	defer func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}()

	job.Status = model.JobRunning
	_ = r.Store.UpdateJob(job)

	cfg, _ := r.Store.GetModelConfig()
	if proj.UseCustomModelConfig && proj.CustomModelConfig != nil && proj.CustomModelConfig.OpenAIAPIKey != "" {
		cfg = *proj.CustomModelConfig
	}

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
			_ = r.Store.UpdateJob(job)
			r.Hub.Publish(jobID, model.JobEvent{Type: "status", Status: string(model.JobCancelled)})
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
	snaps, err := repocontext.Collect(repos, issue.DevSpec.FileChanges)
	if err != nil {
		return err
	}
	_ = r.appendLog(job, "analyzing", fmt.Sprintf("Collected context from %d repo(s)", len(snaps)), "")

	if err := progress(25, "branching", fmt.Sprintf("Creating branch %s", branchName)); err != nil {
		return err
	}
	for _, repo := range repos {
		if err := gitx.CheckoutBranch(repo.Path, repo.DefaultBranch, branchName); err != nil {
			return fmt.Errorf("branch %s: %w", repo.Name, err)
		}
		_ = r.appendLog(job, "branching", fmt.Sprintf("[%s] checked out %s", repo.Name, branchName), "")
	}

	if err := progress(40, "coding", "Generating and applying code changes via LLM..."); err != nil {
		return err
	}
	changes, err := r.generateCode(ctx, cfg, *issue, snaps)
	if err != nil {
		return err
	}
	totalApplied := 0
	for _, repo := range repos {
		n, err := gitx.ApplyFileWrites(repo.Path, changes, repo.Name)
		if err != nil {
			return fmt.Errorf("apply %s: %w", repo.Name, err)
		}
		totalApplied += n
		_ = r.appendLog(job, "coding", fmt.Sprintf("[%s] applied %d file change(s)", repo.Name, n), "")
	}
	if totalApplied == 0 {
		return fmt.Errorf("LLM produced no applicable file changes")
	}

	if err := progress(65, "testing", "Running tests..."); err != nil {
		return err
	}
	for _, repo := range repos {
		out, skipped, err := runTests(repo)
		if skipped {
			_ = r.appendLog(job, "testing", fmt.Sprintf("[%s] no test command detected — skipped", repo.Name), out)
			continue
		}
		if err != nil {
			return fmt.Errorf("tests failed in %s: %w\n%s", repo.Name, err, truncate(out, 1500))
		}
		_ = r.appendLog(job, "testing", fmt.Sprintf("[%s] tests passed", repo.Name), truncate(out, 800))
	}

	if err := progress(80, "linting", "Running lightweight static checks..."); err != nil {
		return err
	}
	for _, repo := range repos {
		out, skipped, err := runLint(repo)
		if skipped {
			_ = r.appendLog(job, "linting", fmt.Sprintf("[%s] lint skipped", repo.Name), "")
			continue
		}
		if err != nil {
			_ = r.appendLog(job, "linting", fmt.Sprintf("[%s] lint warnings (non-blocking): %v", repo.Name, err), truncate(out, 600))
		} else {
			_ = r.appendLog(job, "linting", fmt.Sprintf("[%s] lint ok", repo.Name), truncate(out, 400))
		}
	}

	if err := progress(90, "committing", "Committing changes..."); err != nil {
		return err
	}
	var stats model.DiffStats
	for _, repo := range repos {
		msg := fmt.Sprintf("feat: %s\n\nAuto-generated by Vibecoding for issue %s", issue.Title, issue.ID)
		if err := gitx.CommitAll(repo.Path, msg); err != nil {
			return fmt.Errorf("commit %s: %w", repo.Name, err)
		}
		st, _ := gitx.DiffStats(repo.Path, repo.DefaultBranch)
		stats.Additions += st.Additions
		stats.Deletions += st.Deletions
		stats.FilesChanged += st.FilesChanged
		_ = r.appendLog(job, "committing", fmt.Sprintf("[%s] committed on %s", repo.Name, branchName), "")
	}

	pr := &model.PRInfo{
		ID:          "pr-" + uuid.NewString()[:8],
		BranchName:  branchName,
		Title:       "feat: " + issue.Title,
		Description: "Auto-dev changes generated by Vibecoding (local branch review).",
		Author:      "VibeBot",
		CreatedAt:   model.NowISO(),
		Status:      "open",
		DiffStats:   stats,
	}
	job.Status = model.JobCompleted
	job.Progress = 100
	job.Phase = "completed"
	job.PRInfo = pr
	_ = r.Store.UpdateJob(job)

	issue.Status = model.StatusInReview
	issue.AutoDevProgress = 100
	issue.PRInfo = pr
	issue.UpdatedAt = model.NowISO()
	// Reload logs from DB? Keep existing + append completed
	_ = r.appendLog(job, "completed", "Code committed; issue moved to in_review", "")
	// Refresh issue logs from appended store logs for this job
	logs, _ := r.Store.ListJobLogs(job.ID)
	if len(logs) > 0 {
		issue.AutoDevLogs = append(issue.AutoDevLogs, logs...)
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

func (r *Runner) generateCode(ctx context.Context, cfg model.ModelConfig, issue model.Issue, snaps []repocontext.RepoSnapshot) ([]model.SpecFileChange, error) {
	system := `You are VibeBot, an autonomous coding agent.
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
- Do not wrap JSON in markdown.`

	specJSON, _ := json.Marshal(issue.DevSpec)
	user := fmt.Sprintf("Issue: %s\nDescription: %s\n\nDevSpec JSON:\n%s\n\nRepository context:\n%s\n\nProduce the fileChanges JSON now.",
		issue.Title, issue.Description, string(specJSON), repocontext.FormatForPrompt(snaps))

	text, err := r.LLM.Chat(ctx, llm.ChatRequest{
		ModelConfig: cfg,
		System:      system,
		Messages:    []llm.ChatMessage{{Role: "user", Content: user}},
		Temperature: 0.2,
		JSONMode:    true,
	})
	if err != nil {
		// retry without json mode
		text, err = r.LLM.Chat(ctx, llm.ChatRequest{
			ModelConfig: cfg,
			System:      system,
			Messages:    []llm.ChatMessage{{Role: "user", Content: user}},
			Temperature: 0.2,
		})
		if err != nil {
			return nil, err
		}
	}

	raw := strings.TrimSpace(text)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
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
	var parsed struct {
		FileChanges []model.SpecFileChange `json:"fileChanges"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse coding response: %w", err)
	}
	// If model omitted repoName but only one repo, fill it
	if len(snaps) == 1 {
		for i := range parsed.FileChanges {
			if parsed.FileChanges[i].RepoName == "" {
				parsed.FileChanges[i].RepoName = snaps[0].Name
			}
		}
	}
	return parsed.FileChanges, nil
}

func runTests(repo model.GitRepo) (output string, skipped bool, err error) {
	lang := strings.ToLower(repo.Language + " " + repo.Path)
	switch {
	case fileExists(filepath.Join(repo.Path, "go.mod")) || strings.Contains(lang, "go"):
		return runCmd(repo.Path, "go", "test", "./...")
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
