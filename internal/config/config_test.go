package config

import (
	"context"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
)

func TestLoadReadsDepConfigKeys(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		// args is expected to be: config get <key>
		key := args[len(args)-1]
		switch key {
		case "dep.author":
			return []byte("custom-bot\n"), nil
		case "dep.repo":
			return []byte("g/a, g/b\n"), nil
		case "dep.patterns":
			return []byte(`pat1,pat2`), nil
		}
		return nil, nil
	})

	cfg, err := Load(context.Background(), runner)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Author != "custom-bot" {
		t.Errorf("Author = %q, want %q", cfg.Author, "custom-bot")
	}
	if strings.Join(cfg.Repos, ",") != "g/a,g/b" {
		t.Errorf("Repos = %v, want [g/a g/b]", cfg.Repos)
	}
	if strings.Join(cfg.Patterns, ",") != "pat1,pat2" {
		t.Errorf("Patterns = %v, want [pat1 pat2]", cfg.Patterns)
	}
}

func TestLoadTreatsRunnerErrorsAsUnset(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, context.DeadlineExceeded
	})

	cfg, err := Load(context.Background(), runner)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Author != "" || len(cfg.Repos) != 0 || len(cfg.Patterns) != 0 {
		t.Errorf("expected empty config on runner error, got %+v", cfg)
	}
}
