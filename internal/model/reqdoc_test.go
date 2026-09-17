package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacySpecOnlyStillReady(t *testing.T) {
	iss := &Issue{
		DevSpec: &DevSpec{RawMarkdown: "# old spec", UpdatedAt: "2020-01-01T00:00:00Z"},
	}
	if !iss.LegacySpecOnly() {
		t.Fatal("expected legacy")
	}
	if !iss.RequirementAccepted() {
		t.Fatal("legacy should count as accepted")
	}
	if iss.DesignStale() {
		t.Fatal("legacy should not be stale")
	}
	if !iss.SpecReadyForDev() {
		t.Fatal("legacy with markdown should be ready without fileChanges")
	}
}

func TestReqDocGates(t *testing.T) {
	iss := &Issue{
		ReqDoc: &ReqDoc{
			RawMarkdown: "# req",
			UpdatedAt:   "2024-01-02T00:00:00Z",
		},
		DevSpec: &DevSpec{
			RawMarkdown: "# design",
			UpdatedAt:   "2024-01-01T00:00:00Z",
			FileChanges: []SpecFileChange{{FilePath: "a.go", RepoName: "svc", Action: "modify"}},
		},
	}
	if iss.RequirementAccepted() {
		t.Fatal("unaccepted req should block")
	}
	if iss.SpecReadyForDev() {
		t.Fatal("unaccepted should not be ready")
	}

	iss.AcceptRequirement()
	if !iss.RequirementAccepted() {
		t.Fatal("after accept should be accepted")
	}
	// Req UpdatedAt may be bumped on accept path; force req newer than design.
	iss.ReqDoc.UpdatedAt = "2024-01-03T00:00:00Z"
	iss.ReqDoc.AcceptedAt = "2024-01-03T00:00:00Z"
	if !iss.DesignStale() {
		t.Fatal("req newer than design should be stale")
	}
	if iss.SpecReadyForDev() {
		t.Fatal("stale design should not be ready")
	}

	iss.DevSpec.UpdatedAt = "2024-01-04T00:00:00Z"
	if iss.DesignStale() {
		t.Fatal("fresh design should not be stale")
	}
	if !iss.SpecReadyForDev() {
		t.Fatal("accepted + fresh design should be ready")
	}

	iss.DevSpec.FileChanges = nil
	if !iss.SpecReadyForDev() {
		t.Fatal("empty fileChanges should still allow backlog when markdown design exists")
	}
}

func TestTouchReqDocClearsAcceptance(t *testing.T) {
	iss := &Issue{
		ReqDoc: &ReqDoc{
			RawMarkdown: "# req",
			UpdatedAt:   "2024-01-01T00:00:00Z",
			AcceptedAt:  "2024-01-01T00:00:00Z",
		},
	}
	iss.TouchReqDoc()
	if iss.RequirementAccepted() {
		t.Fatal("edit after accept should require re-accept")
	}
	if iss.DocPhase != DocPhaseRequirement {
		t.Fatalf("phase=%q", iss.DocPhase)
	}
}

func TestVerifyFileChangesCreateAndModify(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exist.go")
	if err := os.WriteFile(existing, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"svc": dir}

	got := VerifyFileChanges([]SpecFileChange{
		{FilePath: "new.go", RepoName: "svc", Action: "create"},
		{FilePath: "exist.go", RepoName: "svc", Action: "modify"},
		{FilePath: "missing.go", RepoName: "svc", Action: "modify"},
		{FilePath: "../escape.go", RepoName: "svc", Action: "create"},
		{FilePath: "x.go", RepoName: "other", Action: "create"},
	}, roots)

	if !got[0].Verified {
		t.Fatal("create in known repo should verify without file existing")
	}
	if !got[1].Verified {
		t.Fatal("modify existing should verify")
	}
	if got[2].Verified {
		t.Fatal("modify missing should not verify")
	}
	if got[3].Verified {
		t.Fatal("path escape should not verify")
	}
	if got[4].Verified {
		t.Fatal("unknown repo create should not verify")
	}
}

func TestHasUnverifiedModifies(t *testing.T) {
	iss := &Issue{
		DevSpec: &DevSpec{
			FileChanges: []SpecFileChange{
				{FilePath: "a.go", Action: "create", Verified: false},
				{FilePath: "b.go", Action: "modify", Verified: false},
			},
		},
	}
	if !iss.HasUnverifiedModifies() {
		t.Fatal("unverified modify should be detected")
	}
	iss.DevSpec.FileChanges[1].Verified = true
	if iss.HasUnverifiedModifies() {
		t.Fatal("only create unverified should not count")
	}
}

func TestSplitSpecReadyWithMarkdownDesigns(t *testing.T) {
	iss := &Issue{
		ReqDoc: &ReqDoc{
			RawMarkdown: "# req",
			UpdatedAt:   "2024-01-01T00:00:00Z",
			AcceptedAt:  "2024-01-01T00:00:00Z",
		},
		SubRequirements: []SubRequirement{
			{
				ID: "a",
				DevSpec: &DevSpec{
					RawMarkdown: "# a",
					UpdatedAt:   "2024-01-02T00:00:00Z",
					FileChanges: []SpecFileChange{{FilePath: "a.go", Action: "modify", Verified: true}},
				},
			},
			{
				ID: "b",
				DevSpec: &DevSpec{
					RawMarkdown: "# b",
					UpdatedAt:   "2024-01-02T00:00:00Z",
				},
			},
		},
	}
	if !iss.SpecReadyForDev() {
		t.Fatal("accepted req + all sub markdown designs should be ready without fileChanges")
	}
	iss.SubRequirements[1].DevSpec.RawMarkdown = ""
	if iss.SpecReadyForDev() {
		t.Fatal("missing sub markdown should block")
	}
}
