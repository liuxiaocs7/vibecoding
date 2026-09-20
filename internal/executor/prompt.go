package executor

import (
	"fmt"
	"strings"

	"github.com/ymhhh/vibecoding/internal/model"
	"github.com/ymhhh/vibecoding/internal/repocontext"
)

func safetyRulesFor(req CodingRequest) string {
	var b strings.Builder
	b.WriteString("Safety rules (mandatory):\n")
	if br := strings.TrimSpace(req.Branch); br != "" {
		base := firstNonEmpty(strings.TrimSpace(req.BaseBranch), "main/master")
		fmt.Fprintf(&b, "- Stay on git branch %q for the entire session. Commit only on that branch.\n", br)
		fmt.Fprintf(&b, "- Do NOT checkout, reset, merge, or commit on the base branch %q (or main/master).\n", base)
		fmt.Fprintf(&b, "- Do NOT run `git checkout -B %s` after committing on another branch — that leaves the commit on the base branch.\n", br)
	} else {
		b.WriteString("- Do NOT merge into the default/main/master branch.\n")
	}
	b.WriteString("- Work only inside the current working directory (isolated git worktree).\n")
	b.WriteString("- Do NOT git push to any remote.\n")
	b.WriteString("- Do NOT modify files outside this worktree.\n")
	b.WriteString("- Prefer small, reviewable commits when you create commits yourself.\n")
	return b.String()
}

// BuildAgentPrompt assembles the natural-language prompt for CLI agents.
func BuildAgentPrompt(req CodingRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are implementing a coding task inside an isolated git worktree.\n")
	fmt.Fprintf(&b, "Repository: %s\n", firstNonEmpty(req.RepoName, "(unnamed)"))
	fmt.Fprintf(&b, "Worktree path: %s\n", req.RepoPath)
	if br := strings.TrimSpace(req.Branch); br != "" {
		fmt.Fprintf(&b, "Current branch (stay on this): %s\n", br)
		if base := strings.TrimSpace(req.BaseBranch); base != "" {
			fmt.Fprintf(&b, "Base / merge-target branch (do not commit here): %s\n", base)
		}
	}
	b.WriteString("\n")
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
	b.WriteString(safetyRulesFor(req))
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
