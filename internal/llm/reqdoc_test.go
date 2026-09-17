package llm

import (
	"strings"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func TestParseReqDocJSON(t *testing.T) {
	raw := `{
		"chatReply":"ok",
		"title":"Feature",
		"summary":"do X",
		"scope":"in",
		"nonGoals":"out",
		"acceptance":"pass",
		"constraints":"mysql",
		"rawMarkdown":"# Feature\n\n## 概述\ndo X"
	}`
	doc, reply, err := ParseReqDocJSON(raw, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "ok" || doc.Title != "Feature" || !strings.Contains(doc.RawMarkdown, "do X") {
		t.Fatalf("%+v reply=%q", doc, reply)
	}
}

func TestParseWantedFilesJSON(t *testing.T) {
	files, _, err := ParseWantedFilesJSON(`{"wantedFiles":[{"repoName":"svc","filePath":"a.go","hintLine":10}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].FilePath != "a.go" || files[0].HintLine != 10 {
		t.Fatalf("%+v", files)
	}
}

func TestParseReqSplitJSONNoDevSpec(t *testing.T) {
	raw := `{
		"chatReply":"split",
		"overviewMarkdown":"# Parent\n\nTwo slices.",
		"subRequirements":[
			{"title":"API","description":"endpoint"},
			{"title":"UI","description":"form"}
		]
	}`
	got, err := ParseReqSplitJSON(raw, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SubRequirements) != 2 {
		t.Fatalf("subs=%d", len(got.SubRequirements))
	}
	for _, sub := range got.SubRequirements {
		if sub.DevSpec != nil {
			t.Fatalf("expected no DevSpec, got %+v", sub.DevSpec)
		}
	}
}

func TestBriefToReqMarkdown(t *testing.T) {
	md := BriefToReqMarkdown(&model.Issue{
		Title:       "Login",
		Description: "Users can sign in",
		Attachments: []model.IssueAttachment{
			{Name: "notes.txt", Text: "SSO only", Kind: "text"},
			{Name: "wire.png", Kind: "image"},
		},
	})
	for _, want := range []string{
		"# Login",
		"## 概述 / Summary",
		"Users can sign in",
		"## 附件原文 / Attached briefs",
		"SSO only",
		"## 附图 / Images",
		"wire.png",
		"## 验收标准 / Acceptance",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in:\n%s", want, md)
		}
	}
}
