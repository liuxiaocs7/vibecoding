package model

import "testing"

func TestPrefixForKind(t *testing.T) {
	cfg := &BranchPrefixConfig{
		FeaturePrefix: "feat/",
		BugfixPrefix:  "fix/",
		HotfixPrefix:  "hotfix/",
	}
	if got := cfg.PrefixForKind(IssueKindFeature); got != "feat/" {
		t.Fatalf("feature=%q", got)
	}
	if got := cfg.PrefixForKind(IssueKindBugfix); got != "fix/" {
		t.Fatalf("bugfix=%q", got)
	}
	if got := cfg.PrefixForKind(IssueKindHotfix); got != "hotfix/" {
		t.Fatalf("hotfix=%q", got)
	}
	if got := cfg.PrefixForKind(""); got != "feat/" {
		t.Fatalf("empty kind defaults to feature, got %q", got)
	}
	if got := (*BranchPrefixConfig)(nil).PrefixForKind(IssueKindBugfix); got != "fix/" {
		t.Fatalf("nil cfg bugfix=%q", got)
	}
}

func TestBranchNameForIssue(t *testing.T) {
	got := BranchNameForIssue("feature/", "ISSUE-90")
	if got != "feature/issue-ISSUE-90" {
		t.Fatalf("got %q", got)
	}
	got = BranchNameForIssue("fix/", "ISSUE-ABCDEF12345678")
	if got != "fix/issue-12345678" {
		t.Fatalf("short id got %q", got)
	}
}

func TestNormalizeIssueKind(t *testing.T) {
	if NormalizeIssueKind("bugfix") != IssueKindBugfix {
		t.Fatal("bugfix")
	}
	if NormalizeIssueKind("hotfix") != IssueKindHotfix {
		t.Fatal("hotfix")
	}
	if NormalizeIssueKind("other") != IssueKindFeature {
		t.Fatal("fallback")
	}
}
