package model

import (
	"strings"
	"testing"
)

func TestPromptDescriptionIncludesAttachmentText(t *testing.T) {
	iss := &Issue{
		Description: "融合孔明与计算智能",
		Attachments: []IssueAttachment{
			{Name: "prd.md", Kind: "text", Text: "# PRD\n方案一"},
			{Name: "ui.png", Kind: "image"},
			{Name: "notes.bin", Kind: "file"},
		},
	}
	got := iss.PromptDescription()
	for _, want := range []string{
		"融合孔明与计算智能",
		"----- Attached file: prd.md -----",
		"# PRD\n方案一",
		"[Attached image: ui.png]",
		"[Attached file: notes.bin]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("PromptDescription missing %q in %q", want, got)
		}
	}
}

func TestPromptDescriptionNilSafe(t *testing.T) {
	var iss *Issue
	if iss.PromptDescription() != "" {
		t.Fatal("nil issue should yield empty prompt description")
	}
}
