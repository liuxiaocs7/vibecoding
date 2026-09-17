package executor

import (
	"fmt"
	"strings"

	"github.com/ymhhh/vibecoding/internal/model"
	"github.com/ymhhh/vibecoding/internal/repocontext"
)

const safetyRules = `Safety rules (mandatory):
- Work only inside the current working directory (isolated git worktree).
- Do NOT git push to any remote.
- Do NOT merge into the default/main/master branch.
- Do NOT modify files outside this worktree.
- Prefer small, reviewable commits when you create commits yourself.`

// BuildAgentPrompt assembles the natural-language prompt for CLI agents.
func BuildAgentPrompt(req CodingRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are implementing a coding task inside an isolated git worktree.\n")
	fmt.Fprintf(&b, "Repository: %s\n", firstNonEmpty(req.RepoName, "(unnamed)"))
	fmt.Fprintf(&b, "Worktree path: %s\n\n", req.RepoPath)
	fmt.Fprintf(&b, "Title: %s\n", req.Title)
	if strings.TrimSpace(req.Description) != "" {
		fmt.Fprintf(&b, "Description:\n%s\n\n", strings.TrimSpace(req.Description))
	}
	if req.Spec != nil {
		md := strings.TrimSpace(req.Spec.RawMarkdown)
		if md == "" {
			md = strings.TrimSpace(req.Spec.Summary)
		}
		if md != "" {
			fmt.Fprintf(&b, "Dev Spec:\n%s\n\n", md)
		}
	}
	if instr := repocontext.InstructionsForPrompt(req.Snapshots); instr != "" {
		fmt.Fprintf(&b, "Repository instructions:\n%s\n\n", instr)
	}
	if extra := strings.TrimSpace(req.Extra); extra != "" {
		fmt.Fprintf(&b, "Additional instructions (scope / repair):\n%s\n\n", extra)
	}
	if resume := strings.TrimSpace(req.Resume); resume != "" {
		fmt.Fprintf(&b, "Resume / prior session note:\n%s\n\n", resume)
	}
	b.WriteString(safetyRules)
	b.WriteString("\n")
	return b.String()
}

// BuildSharedExtra formats sub-requirement / heal context used by both executors.
func BuildSharedExtra(subTitle, subDesc, repairOutput string) string {
	var parts []string
	if strings.TrimSpace(subTitle) != "" {
		parts = append(parts, fmt.Sprintf("Implement ONLY this sub-requirement: %s", strings.TrimSpace(subTitle)))
		if strings.TrimSpace(subDesc) != "" {
			parts = append(parts, strings.TrimSpace(subDesc))
		}
	}
	if strings.TrimSpace(repairOutput) != "" {
		parts = append(parts, "Previous tests failed. Fix the failures using this output:\n"+strings.TrimSpace(repairOutput))
	}
	return strings.Join(parts, "\n\n")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// SpecTitle returns a display title from Spec or fallback.
func SpecTitle(spec *model.DevSpec, fallback string) string {
	if spec != nil && strings.TrimSpace(spec.Title) != "" {
		return strings.TrimSpace(spec.Title)
	}
	return fallback
}
