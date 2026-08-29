package db

import (
	"path/filepath"
	"testing"

	"github.com/ymhhh/vibecoding/internal/model"
)

func TestExecutorConfigDefaultAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	got, err := store.GetExecutorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "llm" || got.MaxHeal != 2 || got.TimeoutSec != 1800 {
		t.Fatalf("defaults: %+v", got)
	}

	cfg := model.ExecutorConfig{
		Type:       "agent",
		Preset:     "claude",
		TimeoutSec: 600,
		MaxHeal:    1,
	}
	if err := store.PutExecutorConfig(cfg); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetExecutorConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "agent" || got.Preset != "claude" || got.TimeoutSec != 600 || got.MaxHeal != 1 {
		t.Fatalf("roundtrip: %+v", got)
	}
}

func TestPutExecutorConfigRejectsBadCustom(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	err = store.PutExecutorConfig(model.ExecutorConfig{
		Type:    "agent",
		Preset:  "custom",
		Command: "my-agent",
		// no {prompt} and no stdin
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	// prior default must remain
	got, _ := store.GetExecutorConfig()
	if got.Type != "llm" {
		t.Fatalf("polluted store: %+v", got)
	}
}

func TestExecutorConfigNormalizeClamps(t *testing.T) {
	cfg := model.ExecutorConfig{Type: "nope", MaxHeal: 99, TimeoutSec: -1}.Normalize()
	if cfg.Type != "llm" || cfg.MaxHeal != 5 || cfg.TimeoutSec != 1800 {
		t.Fatalf("%+v", cfg)
	}
}
