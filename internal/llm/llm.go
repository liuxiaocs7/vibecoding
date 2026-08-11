package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ymhhh/go-common/logger"
	"github.com/ymhhh/vibecoding/internal/model"
)

type Client struct {
	HTTP *http.Client
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 180 * time.Second}}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	ModelConfig     model.ModelConfig
	System          string
	Messages        []ChatMessage
	Temperature     float64
	JSONMode        bool
	OmitTemperature bool // some models (e.g. joybuilder) only accept default temperature
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (string, error) {
	cfg := req.ModelConfig
	start := time.Now()
	provider := ""
	modelName := ""
	var (
		text string
		err  error
	)
	switch {
	case strings.TrimSpace(cfg.OpenAIBaseURL) != "" && strings.TrimSpace(cfg.OpenAIAPIKey) != "":
		provider = "openai"
		modelName = firstNonEmpty(cfg.OpenAIModel, "gpt-4o")
		text, err = c.chatOpenAI(ctx, req)
	case os.Getenv("GEMINI_API_KEY") != "":
		provider = "gemini"
		modelName = "gemini-2.0-flash"
		text, err = c.chatGemini(ctx, os.Getenv("GEMINI_API_KEY"), req)
	default:
		err = fmt.Errorf("no LLM configured: set OpenAPI Base URL + API Key in settings, or GEMINI_API_KEY")
	}
	fields := logger.Fields{
		"provider":    provider,
		"model":       modelName,
		"json_mode":   req.JSONMode,
		"msg_count":   len(req.Messages),
		"duration_ms": time.Since(start).Milliseconds(),
	}
	if provider == "openai" {
		fields["url"] = completionsURL(cfg.OpenAIBaseURL)
	}
	entry := logger.L().WithFields(fields)
	if err != nil {
		entry.WithError(err).Warn("llm chat failed")
		return "", err
	}
	entry.WithField("reply_chars", len(text)).Info("llm chat ok")
	return text, nil
}

func ValidateBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("base URL required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("URL host required")
	}
	// Block obvious SSRF targets for a local app that may later bind beyond localhost.
	if host == "metadata.google.internal" || strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("host not allowed")
	}
	return nil
}

// completionsURL returns the configured chat-completions endpoint as-is (trimmed).
// Callers must store the full path in settings; nothing is appended here.
func completionsURL(base string) string {
	return model.NormalizeOpenAIBaseURL(base)
}

func openAIEndpoint(cfg model.ModelConfig) (url, modelName string) {
	return completionsURL(cfg.OpenAIBaseURL), firstNonEmpty(cfg.OpenAIModel, "gpt-4o")
}

// formatOpenAPIErr includes the request URL and model so failures are diagnosable in UI/logs.
func formatOpenAPIErr(url, modelName string, status int, detail string) error {
	if status > 0 {
		return fmt.Errorf("OpenAPI request failed [%d] url=%s model=%s: %s", status, url, modelName, detail)
	}
	return fmt.Errorf("OpenAPI request failed url=%s model=%s: %s", url, modelName, detail)
}

func (c *Client) chatOpenAI(ctx context.Context, req ChatRequest) (string, error) {
	if err := ValidateBaseURL(req.ModelConfig.OpenAIBaseURL); err != nil {
		return "", err
	}
	endpoint, modelName := openAIEndpoint(req.ModelConfig)
	msgs := make([]ChatMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, ChatMessage{Role: "system", Content: req.System})
	}
	msgs = append(msgs, req.Messages...)

	temp := req.Temperature
	if temp == 0 && req.ModelConfig.Temperature != 0 {
		temp = req.ModelConfig.Temperature
	}
	body := map[string]any{
		"model":    modelName,
		"messages": msgs,
	}
	if !req.OmitTemperature {
		body["temperature"] = temp
	}
	if req.JSONMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}

	doReq := func(payload map[string]any) ([]byte, int, error) {
		b, _ := json.Marshal(payload)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
		if err != nil {
			return nil, 0, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+req.ModelConfig.OpenAIAPIKey)
		resp, err := c.HTTP.Do(httpReq)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		return raw, resp.StatusCode, nil
	}

	payload := body
	tempStripped := req.OmitTemperature
	var (
		raw    []byte
		status int
		err    error
	)
	for attempt := 1; attempt <= openAIMaxAttempts; attempt++ {
		raw, status, err = doReq(payload)
		if err != nil {
			if attempt < openAIMaxAttempts && isRetryableNetErr(err) {
				wait := retryBackoff(attempt, 0)
				logRetry(endpoint, modelName, attempt, 0, wait, err.Error())
				if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
					return "", formatOpenAPIErr(endpoint, modelName, 0, sleepErr.Error())
				}
				continue
			}
			return "", formatOpenAPIErr(endpoint, modelName, 0, err.Error())
		}
		// One-shot: strip temperature for models that only accept the default.
		if status >= 300 && !tempStripped && strings.Contains(strings.ToLower(string(raw)), "temperature") {
			retryBody := map[string]any{
				"model":    body["model"],
				"messages": body["messages"],
			}
			if req.JSONMode {
				retryBody["response_format"] = body["response_format"]
			}
			payload = retryBody
			tempStripped = true
			logRetry(endpoint, modelName, attempt, status, 0, "strip temperature and retry")
			continue
		}
		if status >= 300 && isRetryableHTTPStatus(status) && attempt < openAIMaxAttempts {
			wait := retryBackoff(attempt, status)
			logRetry(endpoint, modelName, attempt, status, wait, truncate(string(raw), 160))
			if sleepErr := sleepCtx(ctx, wait); sleepErr != nil {
				return "", formatOpenAPIErr(endpoint, modelName, status, sleepErr.Error())
			}
			continue
		}
		break
	}
	if err != nil {
		return "", formatOpenAPIErr(endpoint, modelName, 0, err.Error())
	}
	if status >= 300 {
		return "", formatOpenAPIErr(endpoint, modelName, status, truncate(string(raw), 300))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", formatOpenAPIErr(endpoint, modelName, 0, fmt.Sprintf("parse response: %v", err))
	}
	if len(parsed.Choices) == 0 {
		return "", formatOpenAPIErr(endpoint, modelName, 0, "no response generated")
	}
	return parsed.Choices[0].Message.Content, nil
}

func (c *Client) chatGemini(ctx context.Context, apiKey string, req ChatRequest) (string, error) {
	var b strings.Builder
	if req.System != "" {
		b.WriteString(req.System)
		b.WriteString("\n\n")
	}
	for _, m := range req.Messages {
		b.WriteString(strings.ToUpper(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	payload := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": b.String()}}},
		},
	}
	rawBody, _ := json.Marshal(payload)
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + apiKey
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini request failed [%d]: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response generated")
	}
	return parsed.Candidates[0].Content.Parts[0].Text, nil
}

func (c *Client) TestOpenAPI(ctx context.Context, baseURL, apiKey, modelName string) (string, error) {
	if err := ValidateBaseURL(baseURL); err != nil {
		return "", err
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("API key required")
	}
	cfg := model.ModelConfig{
		OpenAIBaseURL: baseURL,
		OpenAIAPIKey:  apiKey,
		OpenAIModel:   firstNonEmpty(modelName, "gpt-4o-mini"),
		Temperature:   1,
	}
	// Omit temperature: some models (e.g. GPT-*-joybuilder) reject 0 and only allow default.
	return c.Chat(ctx, ChatRequest{
		ModelConfig: cfg,
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a test assistant."},
			{Role: "user", Content: "Reply with the word OK if you receive this message."},
		},
		OmitTemperature: true,
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ParseDevSpecJSON extracts a DevSpec (+ optional chatReply) from model JSON
// or treats the whole response as markdown when JSON parsing fails.
func ParseDevSpecJSON(raw string, titleFallback string) (*model.DevSpec, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", fmt.Errorf("empty model response")
	}
	// Strip ```json fences if present.
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```JSON")
		raw = strings.TrimPrefix(raw, "```")
		if i := strings.LastIndex(raw, "```"); i >= 0 {
			raw = raw[:i]
		}
		raw = strings.TrimSpace(raw)
	}
	// Try to find first { ... }
	if !strings.HasPrefix(raw, "{") {
		if i := strings.Index(raw, "{"); i >= 0 {
			if j := strings.LastIndex(raw, "}"); j > i {
				raw = raw[i : j+1]
			}
		}
	}

	var parsed struct {
		ChatReply           string                 `json:"chatReply"`
		Title               string                 `json:"title"`
		Summary             string                 `json:"summary"`
		ArchitectureDesign  string                 `json:"architectureDesign"`
		FileChanges         []model.SpecFileChange `json:"fileChanges"`
		ImplementationSteps []string               `json:"implementationSteps"`
		TestCases           []string               `json:"testCases"`
		RawMarkdown         string                 `json:"rawMarkdown"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// Fallback: treat whole response as markdown-only spec
		return &model.DevSpec{
			Title:               titleFallback,
			Summary:             "Generated from unstructured model output",
			ArchitectureDesign:  "",
			FileChanges:         []model.SpecFileChange{},
			ImplementationSteps: []string{},
			TestCases:           []string{},
			RawMarkdown:         raw,
			UpdatedAt:           model.NowISO(),
		}, "已根据讨论更新待开发文档。", nil
	}
	if parsed.Title == "" {
		parsed.Title = titleFallback
	}
	if parsed.FileChanges == nil {
		parsed.FileChanges = []model.SpecFileChange{}
	}
	if parsed.ImplementationSteps == nil {
		parsed.ImplementationSteps = []string{}
	}
	if parsed.TestCases == nil {
		parsed.TestCases = []string{}
	}
	md := strings.TrimSpace(parsed.RawMarkdown)
	if md == "" {
		md = buildMarkdown(parsed.Title, parsed.Summary, parsed.ArchitectureDesign, parsed.FileChanges, parsed.ImplementationSteps, parsed.TestCases)
	}
	// Normalize: ensure document starts with a title heading when missing.
	if parsed.Title != "" && !strings.HasPrefix(md, "#") {
		md = "# " + parsed.Title + "\n\n" + md
	}
	chatReply := strings.TrimSpace(parsed.ChatReply)
	if chatReply == "" {
		chatReply = "已更新待开发文档，可在「待开发文档」页查看。"
	}
	return &model.DevSpec{
		Title:               parsed.Title,
		Summary:             parsed.Summary,
		ArchitectureDesign:  parsed.ArchitectureDesign,
		FileChanges:         parsed.FileChanges,
		ImplementationSteps: parsed.ImplementationSteps,
		TestCases:           parsed.TestCases,
		RawMarkdown:         md,
		UpdatedAt:           model.NowISO(),
	}, chatReply, nil
}

func buildMarkdown(title, summary, arch string, files []model.SpecFileChange, steps, tests []string) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n## Executive Summary\n")
	b.WriteString(summary)
	b.WriteString("\n\n## Architecture Design\n")
	b.WriteString(arch)
	b.WriteString("\n\n## Target Files & Changes\n")
	for _, f := range files {
		b.WriteString(fmt.Sprintf("- [%s] %s/%s — %s\n", f.Action, f.RepoName, f.FilePath, f.Summary))
	}
	b.WriteString("\n## Implementation Steps\n")
	for i, s := range steps {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
	}
	b.WriteString("\n## Test Cases\n")
	for _, t := range tests {
		b.WriteString("- ")
		b.WriteString(t)
		b.WriteString("\n")
	}
	return b.String()
}
