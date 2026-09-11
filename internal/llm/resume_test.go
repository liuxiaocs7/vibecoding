package llm

import (
	"strings"
	"testing"
)

func TestApplyResumeHintFresh(t *testing.T) {
	got := ApplyResumeHint("分析开发方案", "", true)
	if !strings.Contains(got, "分析开发方案") || !strings.Contains(got, "Regenerate") {
		t.Fatalf("fresh hint missing: %q", got)
	}
}

func TestApplyResumeHintSessionPartial(t *testing.T) {
	got := ApplyResumeHint("分析开发方案", `{"title":"draft"}`, false)
	if !strings.Contains(got, "分析开发方案") || !strings.Contains(got, "incomplete draft") || !strings.Contains(got, `{"title":"draft"}`) {
		t.Fatalf("session hint missing: %q", got)
	}
}

func TestApplyResumeHintTimeoutNoDraft(t *testing.T) {
	got := ApplyResumeHint("分析开发方案", "", false)
	if !strings.Contains(got, "Retry the same request") {
		t.Fatalf("empty-draft retry hint missing: %q", got)
	}
}
