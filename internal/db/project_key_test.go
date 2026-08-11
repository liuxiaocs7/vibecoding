package db

import (
	"path/filepath"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func TestUpsertProjectPreservesCustomAPIKey(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cfg := model.DefaultModelConfig()
	cfg.OpenAIAPIKey = "secret-key-1234"
	cfg.OpenAIBaseURL = "http://example.com/v1/chat/completions"
	cfg.OpenAIModel = "test-model"
	p := model.Project{
		ID:                   "proj-test",
		Name:                 "t",
		UseCustomModelConfig: true,
		CustomModelConfig:    &cfg,
		CreatedAt:            model.NowISO(),
	}
	if err := store.UpsertProject(p); err != nil {
		t.Fatal(err)
	}

	// Client re-saves after a Public() response (blank key + keyConfigured/hint).
	pub := cfg.Public()
	p2 := model.Project{
		ID:                   "proj-test",
		Name:                 "t-renamed",
		UseCustomModelConfig: true,
		CustomModelConfig:    &pub,
		CreatedAt:            p.CreatedAt,
	}
	if err := store.UpsertProject(p2); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetProject("proj-test")
	if err != nil || got == nil || got.CustomModelConfig == nil {
		t.Fatalf("get project: %v %#v", err, got)
	}
	if got.CustomModelConfig.OpenAIAPIKey != "secret-key-1234" {
		t.Fatalf("key not preserved: %q", got.CustomModelConfig.OpenAIAPIKey)
	}
	if got.CustomModelConfig.KeyConfigured || got.CustomModelConfig.KeyHint != "" {
		t.Fatalf("response-only fields should not be persisted: %+v", got.CustomModelConfig)
	}
	if got.Name != "t-renamed" {
		t.Fatalf("name not updated: %q", got.Name)
	}
}
