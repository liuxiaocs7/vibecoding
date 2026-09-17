package llm

import (
	"strings"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func TestParseSplitJSON(t *testing.T) {
	raw := `{
		"chatReply": "Split into two slices.",
		"overviewMarkdown": "# Parent\n\nTwo steps.",
		"title": "Parent",
		"subRequirements": [
			{"id":"sub-a","title":"API","description":"add endpoint","rawMarkdown":"# API\n\nDo it.","fileChanges":[{"filePath":"a.go","repoName":"svc","action":"modify","summary":"handler"}]},
			{"title":"UI","rawMarkdown":"# UI\n\nForm."}
		]
	}`
	got, err := ParseSplitJSON(raw, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	if got.ChatReply != "Split into two slices." {
		t.Fatalf("chatReply=%q", got.ChatReply)
	}
	if got.OverviewSpec == nil || !strings.Contains(got.OverviewSpec.RawMarkdown, "Two steps") {
		t.Fatalf("overview missing: %+v", got.OverviewSpec)
	}
	if len(got.SubRequirements) != 2 {
		t.Fatalf("subs=%d", len(got.SubRequirements))
	}
	if got.SubRequirements[0].ID != "sub-a" || got.SubRequirements[0].Order != 1 {
		t.Fatalf("first sub: %+v", got.SubRequirements[0])
	}
	if got.SubRequirements[1].Title != "UI" || got.SubRequirements[1].DevSpec == nil {
		t.Fatalf("second sub: %+v", got.SubRequirements[1])
	}
}

func TestApplyMultiSubSpec(t *testing.T) {
	iss := &model.Issue{
		Title: "Parent",
		SubRequirements: []model.SubRequirement{
			{ID: "sub-a", Title: "API", Order: 1, Status: model.SubReqReady, DevSpec: &model.DevSpec{RawMarkdown: "# old a"}},
			{ID: "sub-b", Title: "UI", Order: 2, Status: model.SubReqReady, DevSpec: &model.DevSpec{RawMarkdown: "# old b"}},
		},
	}
	parsed, err := ParseMultiSubSpecJSON(`{
		"chatReply": "Updated both.",
		"overviewMarkdown": "# Parent overview",
		"subRequirements": [
			{"id":"sub-b","title":"UI v2","rawMarkdown":"# UI v2"},
			{"id":"sub-a","title":"API v2","rawMarkdown":"# API v2"}
		]
	}`, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	ApplyMultiSubSpec(iss, parsed)
	if iss.DevSpec == nil || !strings.Contains(iss.DevSpec.RawMarkdown, "overview") {
		t.Fatalf("overview: %+v", iss.DevSpec)
	}
	if iss.SubRequirements[0].DevSpec.RawMarkdown != "# API v2" {
		t.Fatalf("sub-a spec=%q", iss.SubRequirements[0].DevSpec.RawMarkdown)
	}
	if iss.SubRequirements[1].Title != "UI v2" {
		t.Fatalf("sub-b title=%q", iss.SubRequirements[1].Title)
	}
}

func TestSpecReadyForDev(t *testing.T) {
	plain := &model.Issue{}
	if plain.SpecReadyForDev() {
		t.Fatal("empty issue should not be ready")
	}
	plain.DevSpec = &model.DevSpec{RawMarkdown: "# spec"}
	if !plain.SpecReadyForDev() {
		t.Fatal("legacy parent spec should be ready")
	}
	split := &model.Issue{
		DevSpec: &model.DevSpec{RawMarkdown: "# overview"},
		SubRequirements: []model.SubRequirement{
			{ID: "a", DevSpec: &model.DevSpec{RawMarkdown: "# a"}},
			{ID: "b", DevSpec: &model.DevSpec{RawMarkdown: ""}},
		},
	}
	if split.SpecReadyForDev() {
		t.Fatal("incomplete subs should not be ready")
	}
	split.SubRequirements[1].DevSpec.RawMarkdown = "# b"
	if !split.SpecReadyForDev() {
		t.Fatal("legacy all subs with specs should be ready")
	}
}
