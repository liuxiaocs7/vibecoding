package api

import (
	"fmt"
	"strings"

	"github.com/ymhhh/vibecoding/internal/model"
	"github.com/ymhhh/vibecoding/internal/repocontext"
)

// resolveChatRepos prefers store-backed associated repos when issueId is set.
func (s *Server) resolveChatRepos(projectID, issueID string, refs []repoRef) []model.GitRepo {
	if issueID != "" {
		if issue, _ := s.Store.GetIssue(issueID); issue != nil {
			pid := projectID
			if pid == "" {
				pid = issue.ProjectID
			}
			if proj, _ := s.Store.GetProject(pid); proj != nil {
				if repos := s.associatedRepos(issue, proj); len(repos) > 0 {
					return repos
				}
			}
		}
	}
	out := make([]model.GitRepo, 0, len(refs))
	for _, r := range refs {
		name := strings.TrimSpace(r.Name)
		path := strings.TrimSpace(r.Path)
		if path == "" {
			continue
		}
		if name == "" {
			name = path
		}
		out = append(out, model.GitRepo{
			Name:          name,
			Path:          path,
			DefaultBranch: r.DefaultBranch,
		})
	}
	return out
}

// loadRepoExcerptsForQuery indexes associated repos and reads top candidate windows.
func loadRepoExcerptsForQuery(repos []model.GitRepo, query string) (excerptBlock string, nFiles int, err error) {
	if len(repos) == 0 {
		return "", 0, nil
	}
	ex, _, err := repocontext.AutoExcerpts(repos, query, 8)
	if err != nil {
		return "", 0, err
	}
	n := repocontext.ExcerptFileCount(ex)
	if n == 0 {
		// Still provide the index so the model knows what exists.
		idx, idxErr := repocontext.BuildDesignIndex(repos, query)
		if idxErr != nil {
			return "", 0, idxErr
		}
		block := "Associated repositories were scanned locally, but no strong file hits matched this query.\n" +
			"Candidate paths (you MAY cite these when discussing naming/structure; ask to open a specific path if needed):\n\n" +
			repocontext.FormatDesignIndexForPrompt(idx)
		return block, 0, nil
	}
	return repocontext.FormatDesignExcerptsForPrompt(ex), n, nil
}

func chatQueryText(title, description, prompt string, recent []model.ChatMessage) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(description)
	b.WriteString("\n")
	b.WriteString(prompt)
	// Fold a few recent user turns so naming / field keywords score.
	n := 0
	for i := len(recent) - 1; i >= 0 && n < 4; i-- {
		m := recent[i]
		if m.Sender != "user" {
			continue
		}
		b.WriteString("\n")
		b.WriteString(m.Text)
		n++
	}
	return b.String()
}

func formatRepoList(repos []model.GitRepo) string {
	if len(repos) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(repos))
	for _, r := range repos {
		parts = append(parts, fmt.Sprintf("%s @ %s", r.Name, r.Path))
	}
	return strings.Join(parts, "; ")
}
