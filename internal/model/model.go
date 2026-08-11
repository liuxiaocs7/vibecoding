package model

import (
	"strings"
	"time"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

type IssueStatus string

const (
	StatusRequirements IssueStatus = "requirements"
	StatusBacklog      IssueStatus = "backlog"
	StatusInProgress   IssueStatus = "in_progress"
	StatusInReview     IssueStatus = "in_review"
	StatusCompleted    IssueStatus = "completed"
)

type GitRepo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	DefaultBranch string `json:"defaultBranch"`
	Language      string `json:"language"`
	Description   string `json:"description,omitempty"`
	FilesCount    int    `json:"filesCount,omitempty"`
	URL           string `json:"url,omitempty"`
}

type ModelConfig struct {
	UseCustomOpenAI bool    `json:"useCustomOpenAI"`
	OpenAIBaseURL   string  `json:"openAIBaseUrl"`
	OpenAIAPIKey    string  `json:"openAIApiKey"`
	OpenAIModel     string  `json:"openAIModel"`
	Temperature     float64 `json:"temperature"`
	// KeyConfigured is set on API responses when a key exists server-side.
	KeyConfigured bool   `json:"keyConfigured,omitempty"`
	KeyHint       string `json:"keyHint,omitempty"`
}

type BranchPrefixConfig struct {
	FeaturePrefix  string `json:"featurePrefix"`
	BugfixPrefix  string `json:"bugfixPrefix"`
	HotfixPrefix  string `json:"hotfixPrefix"`
	RefactorPrefix string `json:"refactorPrefix"`
	AutoDevPrefix  string `json:"autoDevPrefix"`
	ReleasePrefix  string `json:"releasePrefix"`
}

func DefaultBranchPrefix() BranchPrefixConfig {
	return BranchPrefixConfig{
		FeaturePrefix:  "feature/",
		BugfixPrefix:  "fix/",
		HotfixPrefix:  "hotfix/",
		RefactorPrefix: "refactor/",
		AutoDevPrefix:  "ai-dev/",
		ReleasePrefix:  "release/",
	}
}

func DefaultModelConfig() ModelConfig {
	return ModelConfig{
		UseCustomOpenAI: true,
		OpenAIBaseURL:   "https://api.openai.com/v1/chat/completions",
		OpenAIAPIKey:    "",
		OpenAIModel:     "gpt-4o",
		Temperature:     0.7,
	}
}

// NormalizeOpenAIBaseURL trims the configured chat-completions URL.
// The value is used as-is for HTTP calls (no path is appended in the client).
func NormalizeOpenAIBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

// UpgradeLegacyOpenAIBaseURL migrates old API-root values (e.g. ".../v1") to the
// full chat completions path so existing installs keep working after the UI
// started requiring a complete endpoint URL.
func UpgradeLegacyOpenAIBaseURL(raw string) string {
	u := NormalizeOpenAIBaseURL(raw)
	if u == "" {
		return u
	}
	if strings.Contains(strings.ToLower(u), "/chat/completions") {
		return u
	}
	return u + "/chat/completions"
}

type Project struct {
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Description         string              `json:"description"`
	GitRepos           []GitRepo           `json:"gitRepos"`
	BranchPrefixConfig  *BranchPrefixConfig `json:"branchPrefixConfig,omitempty"`
	CustomModelConfig   *ModelConfig        `json:"customModelConfig,omitempty"`
	UseCustomModelConfig bool               `json:"useCustomModelConfig"`
	CreatedAt           string              `json:"createdAt"`
	UpdatedAt           string              `json:"updatedAt"`
}

type ChatMessage struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"` // user | ai | system
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

type SpecFileChange struct {
	FilePath     string `json:"filePath"`
	RepoName     string `json:"repoName"`
	Action       string `json:"action"` // create | modify | delete
	Summary      string `json:"summary"`
	OriginalCode string `json:"originalCode,omitempty"`
	ModifiedCode string `json:"modifiedCode,omitempty"`
}

type DevSpec struct {
	Title               string           `json:"title"`
	Summary             string           `json:"summary"`
	ArchitectureDesign  string           `json:"architectureDesign"`
	FileChanges         []SpecFileChange `json:"fileChanges"`
	ImplementationSteps []string         `json:"implementationSteps"`
	TestCases           []string         `json:"testCases"`
	RawMarkdown         string           `json:"rawMarkdown"`
	UpdatedAt           string           `json:"updatedAt"`
}

type AutoDevLog struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Phase     string `json:"phase"`
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
}

type DiffStats struct {
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	FilesChanged int `json:"filesChanged"`
}

type PRInfo struct {
	ID          string    `json:"id"`
	BranchName  string    `json:"branchName"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	CreatedAt   string    `json:"createdAt"`
	Status      string    `json:"status"` // open | merged | rejected
	DiffStats   DiffStats `json:"diffStats"`
}

type Issue struct {
	ID                string       `json:"id"`
	ProjectID         string       `json:"projectId"`
	Title             string       `json:"title"`
	Description       string       `json:"description"`
	Priority          Priority     `json:"priority"`
	Status            IssueStatus  `json:"status"`
	AssociatedRepoIDs []string     `json:"associatedRepoIds"`
	Assignee          string       `json:"assignee"`
	ChatMessages      []ChatMessage `json:"chatMessages"`
	DevSpec           *DevSpec     `json:"devSpec,omitempty"`
	AutoDevLogs       []AutoDevLog `json:"autoDevLogs"`
	AutoDevProgress   int          `json:"autoDevProgress"`
	PRInfo            *PRInfo      `json:"prInfo,omitempty"`
	ReviewFeedback    string       `json:"reviewFeedback,omitempty"`
	CreatedAt         string       `json:"createdAt"`
	UpdatedAt         string       `json:"updatedAt"`
}

type UIPrefs struct {
	Language        string `json:"language"`
	ThemeStyle      string `json:"themeStyle"`
	ActiveProjectID string `json:"activeProjectId"`
}

type AutoDevJobStatus string

const (
	JobQueued     AutoDevJobStatus = "queued"
	JobRunning    AutoDevJobStatus = "running"
	JobCompleted  AutoDevJobStatus = "completed"
	JobFailed     AutoDevJobStatus = "failed"
	JobCancelled  AutoDevJobStatus = "cancelled"
)

type AutoDevJob struct {
	ID        string           `json:"id"`
	IssueID   string           `json:"issueId"`
	Status    AutoDevJobStatus `json:"status"`
	Progress  int              `json:"progress"`
	Phase     string           `json:"phase,omitempty"`
	Error     string           `json:"error,omitempty"`
	PRInfo    *PRInfo          `json:"prInfo,omitempty"`
	CreatedAt string           `json:"createdAt"`
	UpdatedAt string           `json:"updatedAt"`
}

type JobEvent struct {
	Type     string `json:"type"` // log | progress | status | done | error
	Phase    string `json:"phase,omitempty"`
	Message  string `json:"message,omitempty"`
	Details  string `json:"details,omitempty"`
	Progress int    `json:"progress,omitempty"`
	Status   string `json:"status,omitempty"`
	Log      *AutoDevLog `json:"log,omitempty"`
	PRInfo   *PRInfo `json:"prInfo,omitempty"`
	Error    string `json:"error,omitempty"`
}

func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func MaskKey(key string) (configured bool, hint string) {
	key = trimSpace(key)
	if key == "" {
		return false, ""
	}
	if len(key) <= 4 {
		return true, "****"
	}
	return true, "****" + key[len(key)-4:]
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

// PublicModel strips secrets for API responses.
func (m ModelConfig) Public() ModelConfig {
	out := m
	configured, hint := MaskKey(m.OpenAIAPIKey)
	out.OpenAIAPIKey = ""
	out.KeyConfigured = configured
	out.KeyHint = hint
	return out
}
