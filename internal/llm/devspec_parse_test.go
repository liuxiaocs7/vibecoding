package llm

import (
	"strings"
	"testing"
)

func TestParseDevSpecJSONExtractsMarkdown(t *testing.T) {
	raw := `{
		"chatReply":"ok",
		"title":"Design",
		"summary":"sum",
		"architectureDesign":"arch",
		"rawMarkdown":"# Design\n\n## Architecture\nok",
		"fileChanges":[{"filePath":"a.go","repoName":"svc","action":"modify","summary":"x"}]
	}`
	spec, reply, err := ParseDevSpecJSON(raw, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "ok" || spec.Title != "Design" {
		t.Fatalf("%+v reply=%q", spec, reply)
	}
	if !strings.Contains(spec.RawMarkdown, "# Design") || strings.Contains(spec.RawMarkdown, `"chatReply"`) {
		t.Fatalf("md=%s", spec.RawMarkdown)
	}
	if len(spec.FileChanges) != 1 {
		t.Fatalf("fileChanges=%d", len(spec.FileChanges))
	}
}

func TestParseDevSpecJSONTrailingCommaEnvelope(t *testing.T) {
	raw := `{
		"chatReply": "已生成",
		"title": "审计设计",
		"rawMarkdown": "# 审计设计\n\n## Summary\ntrace\n",
	}`
	spec, reply, err := ParseDevSpecJSON(raw, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "已生成" {
		t.Fatalf("reply=%q", reply)
	}
	if !strings.Contains(spec.RawMarkdown, "# 审计设计") || strings.HasPrefix(strings.TrimSpace(spec.RawMarkdown), "{") {
		t.Fatalf("md=%s", spec.RawMarkdown)
	}
}

func TestParseDevSpecJSONRawNewlinesInString(t *testing.T) {
	raw := "{\n  \"chatReply\": \"ok\",\n  \"title\": \"T\",\n  \"rawMarkdown\": \"# T\n\n## Arch\n\nbody\"\n}"
	spec, _, err := ParseDevSpecJSON(raw, "Fallback")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(spec.RawMarkdown, "# T") || !strings.Contains(spec.RawMarkdown, "body") {
		t.Fatalf("md=%s", spec.RawMarkdown)
	}
	if strings.Contains(spec.RawMarkdown, `"rawMarkdown"`) {
		t.Fatalf("envelope leaked: %s", spec.RawMarkdown)
	}
}
