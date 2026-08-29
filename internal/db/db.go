package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ymhhh/vibecoding/internal/model"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(1)
	s := &Store{db: sqldb}
	if err := s.migrate(); err != nil {
		_ = sqldb.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  data TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS issues (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  data TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS autodev_jobs (
  id TEXT PRIMARY KEY,
  issue_id TEXT NOT NULL,
  status TEXT NOT NULL,
  progress INTEGER NOT NULL DEFAULT 0,
  phase TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  pr_info TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS autodev_logs (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL,
  issue_id TEXT NOT NULL,
  data TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_issues_project ON issues(project_id);
CREATE INDEX IF NOT EXISTS idx_jobs_issue ON autodev_jobs(issue_id);
CREATE INDEX IF NOT EXISTS idx_logs_job ON autodev_logs(job_id);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) GetSetting(key string, dest any) (bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if dest == nil {
		return true, nil
	}
	return true, json.Unmarshal([]byte(raw), dest)
}

func (s *Store) PutSetting(key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO settings(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value
`, key, string(b))
	return err
}

func (s *Store) GetModelConfig() (model.ModelConfig, error) {
	cfg := model.DefaultModelConfig()
	ok, err := s.GetSetting("global_model_config", &cfg)
	if err != nil {
		return cfg, err
	}
	if !ok {
		return model.DefaultModelConfig(), nil
	}
	if upgraded := upgradeModelConfigURLs(&cfg); upgraded {
		// Persist migrated full endpoint so the UI shows the real path.
		_ = s.PutSetting("global_model_config", cfg)
	}
	return cfg, nil
}

func (s *Store) PutModelConfig(cfg model.ModelConfig) error {
	existing, _ := s.GetModelConfig()
	// Preserve existing key when client sends empty (masked update).
	if strings.TrimSpace(cfg.OpenAIAPIKey) == "" && existing.OpenAIAPIKey != "" {
		cfg.OpenAIAPIKey = existing.OpenAIAPIKey
	}
	upgradeModelConfigURLs(&cfg)
	return s.PutSetting("global_model_config", cfg)
}

func upgradeModelConfigURLs(cfg *model.ModelConfig) bool {
	if cfg == nil {
		return false
	}
	next := model.UpgradeLegacyOpenAIBaseURL(cfg.OpenAIBaseURL)
	if next == cfg.OpenAIBaseURL {
		return false
	}
	cfg.OpenAIBaseURL = next
	return true
}

func (s *Store) GetUIPrefs() (model.UIPrefs, error) {
	prefs := model.UIPrefs{Language: "en", ThemeStyle: "glass"}
	ok, err := s.GetSetting("ui_prefs", &prefs)
	if err != nil {
		return prefs, err
	}
	if !ok {
		return prefs, nil
	}
	return prefs, nil
}

func (s *Store) PutUIPrefs(prefs model.UIPrefs) error {
	return s.PutSetting("ui_prefs", prefs)
}

func (s *Store) ListProjects() ([]model.Project, error) {
	rows, err := s.db.Query(`SELECT data FROM projects ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Project
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var p model.Project
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return nil, err
		}
		if p.CustomModelConfig != nil {
			upgradeModelConfigURLs(p.CustomModelConfig)
		}
		out = append(out, p)
	}
	if out == nil {
		out = []model.Project{}
	}
	return out, rows.Err()
}

func (s *Store) GetProject(id string) (*model.Project, error) {
	var raw string
	err := s.db.QueryRow(`SELECT data FROM projects WHERE id = ?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p model.Project
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	if p.CustomModelConfig != nil {
		upgradeModelConfigURLs(p.CustomModelConfig)
	}
	return &p, nil
}

func (s *Store) UpsertProject(p model.Project) error {
	if p.ID == "" {
		p.ID = "proj-" + uuid.NewString()[:8]
	}
	now := model.NowISO()
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if p.GitRepos == nil {
		p.GitRepos = []model.GitRepo{}
	}

	// Preserve project custom API key when the client sends a masked/blank value.
	// Also drop response-only fields (keyConfigured/keyHint) so they are not persisted.
	if existing, _ := s.GetProject(p.ID); existing != nil {
		p.CustomModelConfig = mergeCustomModelConfig(existing.CustomModelConfig, p.CustomModelConfig)
	} else {
		p.CustomModelConfig = sanitizeModelConfigForStore(p.CustomModelConfig)
	}

	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO projects(id, data, created_at, updated_at) VALUES(?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at
`, p.ID, string(b), p.CreatedAt, p.UpdatedAt)
	return err
}

func mergeCustomModelConfig(existing, incoming *model.ModelConfig) *model.ModelConfig {
	if incoming == nil {
		// Client omitted custom config — keep whatever was stored.
		return sanitizeModelConfigForStore(existing)
	}
	out := *incoming
	if strings.TrimSpace(out.OpenAIAPIKey) == "" && existing != nil {
		out.OpenAIAPIKey = existing.OpenAIAPIKey
	}
	return sanitizeModelConfigForStore(&out)
}

func sanitizeModelConfigForStore(cfg *model.ModelConfig) *model.ModelConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.KeyConfigured = false
	out.KeyHint = ""
	out.OpenAIBaseURL = model.UpgradeLegacyOpenAIBaseURL(out.OpenAIBaseURL)
	return &out
}

func (s *Store) DeleteProject(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM issues WHERE project_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM projects WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListIssues(projectID string) ([]model.Issue, error) {
	var rows *sql.Rows
	var err error
	if projectID == "" {
		rows, err = s.db.Query(`SELECT data FROM issues ORDER BY updated_at DESC`)
	} else {
		rows, err = s.db.Query(`SELECT data FROM issues WHERE project_id = ? ORDER BY updated_at DESC`, projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Issue
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var iss model.Issue
		if err := json.Unmarshal([]byte(raw), &iss); err != nil {
			return nil, err
		}
		if iss.ChatMessages == nil {
			iss.ChatMessages = []model.ChatMessage{}
		}
		if iss.AutoDevLogs == nil {
			iss.AutoDevLogs = []model.AutoDevLog{}
		}
		if iss.AssociatedRepoIDs == nil {
			iss.AssociatedRepoIDs = []string{}
		}
		iss.NormalizeSubs()
		out = append(out, iss)
	}
	if out == nil {
		out = []model.Issue{}
	}
	return out, rows.Err()
}

func (s *Store) GetIssue(id string) (*model.Issue, error) {
	var raw string
	err := s.db.QueryRow(`SELECT data FROM issues WHERE id = ?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var iss model.Issue
	if err := json.Unmarshal([]byte(raw), &iss); err != nil {
		return nil, err
	}
	if iss.ChatMessages == nil {
		iss.ChatMessages = []model.ChatMessage{}
	}
	if iss.AutoDevLogs == nil {
		iss.AutoDevLogs = []model.AutoDevLog{}
	}
	if iss.AssociatedRepoIDs == nil {
		iss.AssociatedRepoIDs = []string{}
	}
	iss.NormalizeSubs()
	return &iss, nil
}

func (s *Store) UpsertIssue(iss model.Issue) error {
	if iss.ID == "" {
		iss.ID = "issue-" + uuid.NewString()[:8]
	}
	now := model.NowISO()
	if iss.CreatedAt == "" {
		iss.CreatedAt = now
	}
	iss.UpdatedAt = now
	if iss.ChatMessages == nil {
		iss.ChatMessages = []model.ChatMessage{}
	}
	if iss.AutoDevLogs == nil {
		iss.AutoDevLogs = []model.AutoDevLog{}
	}
	if iss.AssociatedRepoIDs == nil {
		iss.AssociatedRepoIDs = []string{}
	}
	iss.NormalizeSubs()
	b, err := json.Marshal(iss)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO issues(id, project_id, data, status, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET data = excluded.data, project_id = excluded.project_id, status = excluded.status, updated_at = excluded.updated_at
`, iss.ID, iss.ProjectID, string(b), string(iss.Status), iss.CreatedAt, iss.UpdatedAt)
	return err
}

func (s *Store) DeleteIssue(id string) error {
	_, err := s.db.Exec(`DELETE FROM issues WHERE id = ?`, id)
	return err
}

func (s *Store) CreateJob(issueID string) (*model.AutoDevJob, error) {
	now := model.NowISO()
	job := &model.AutoDevJob{
		ID:        "job-" + uuid.NewString()[:12],
		IssueID:   issueID,
		Status:    model.JobQueued,
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(`
INSERT INTO autodev_jobs(id, issue_id, status, progress, phase, error, pr_info, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, NULL, ?, ?)
`, job.ID, job.IssueID, job.Status, job.Progress, "", "", job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) GetJob(id string) (*model.AutoDevJob, error) {
	var job model.AutoDevJob
	var prRaw sql.NullString
	err := s.db.QueryRow(`
SELECT id, issue_id, status, progress, phase, error, pr_info, created_at, updated_at
FROM autodev_jobs WHERE id = ?
`, id).Scan(&job.ID, &job.IssueID, &job.Status, &job.Progress, &job.Phase, &job.Error, &prRaw, &job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if prRaw.Valid && prRaw.String != "" {
		var pr model.PRInfo
		if err := json.Unmarshal([]byte(prRaw.String), &pr); err == nil {
			job.PRInfo = &pr
		}
	}
	return &job, nil
}

func (s *Store) UpdateJob(job *model.AutoDevJob) error {
	job.UpdatedAt = model.NowISO()
	var pr any
	if job.PRInfo != nil {
		b, _ := json.Marshal(job.PRInfo)
		pr = string(b)
	}
	_, err := s.db.Exec(`
UPDATE autodev_jobs SET status=?, progress=?, phase=?, error=?, pr_info=?, updated_at=? WHERE id=?
`, job.Status, job.Progress, job.Phase, job.Error, pr, job.UpdatedAt, job.ID)
	return err
}

func (s *Store) ActiveJobForRepoIssue(issueID string) (*model.AutoDevJob, error) {
	var id string
	err := s.db.QueryRow(`
SELECT id FROM autodev_jobs
WHERE issue_id = ? AND status IN ('queued','running')
ORDER BY created_at DESC LIMIT 1
`, issueID).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.GetJob(id)
}

func (s *Store) AppendJobLog(jobID, issueID string, log model.AutoDevLog) error {
	if log.ID == "" {
		log.ID = "log-" + uuid.NewString()[:10]
	}
	if log.Timestamp == "" {
		log.Timestamp = time.Now().Format("15:04:05")
	}
	b, err := json.Marshal(log)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO autodev_logs(id, job_id, issue_id, data, created_at) VALUES(?, ?, ?, ?, ?)
`, log.ID, jobID, issueID, string(b), model.NowISO())
	return err
}

func (s *Store) ListJobLogs(jobID string) ([]model.AutoDevLog, error) {
	rows, err := s.db.Query(`SELECT data FROM autodev_logs WHERE job_id = ? ORDER BY created_at ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.AutoDevLog
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var log model.AutoDevLog
		if err := json.Unmarshal([]byte(raw), &log); err != nil {
			return nil, err
		}
		out = append(out, log)
	}
	if out == nil {
		out = []model.AutoDevLog{}
	}
	return out, rows.Err()
}

func (s *Store) GetExecutorConfig() (model.ExecutorConfig, error) {
	cfg := model.DefaultExecutorConfig()
	ok, err := s.GetSetting("executor_config", &cfg)
	if err != nil {
		return cfg, err
	}
	if !ok {
		return model.DefaultExecutorConfig(), nil
	}
	return cfg.Normalize(), nil
}

func (s *Store) PutExecutorConfig(cfg model.ExecutorConfig) error {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	return s.PutSetting("executor_config", cfg)
}
