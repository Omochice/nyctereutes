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
			// Newline-separated; the second pattern contains a comma that must
			// survive (regex quantifier).
			return []byte("foo.*\na{1,3}"), nil
		}
		return nil, nil
	})

	cfg := Load(context.Background(), runner)

	if cfg.Author != "custom-bot" {
		t.Errorf("Author = %q, want %q", cfg.Author, "custom-bot")
	}
	if strings.Join(cfg.Repos, ",") != "g/a,g/b" {
		t.Errorf("Repos = %v, want [g/a g/b]", cfg.Repos)
	}
	if len(cfg.Patterns) != 2 || cfg.Patterns[0] != "foo.*" || cfg.Patterns[1] != "a{1,3}" {
		t.Errorf("Patterns = %v, want [foo.* a{1,3}] with the comma preserved", cfg.Patterns)
	}
}

func TestLoadTreatsRunnerErrorsAsUnset(t *testing.T) {
	runner := glab.RunnerFunc(func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, context.DeadlineExceeded
	})

	cfg := Load(context.Background(), runner)
	if cfg.Author != "" || len(cfg.Repos) != 0 || len(cfg.Patterns) != 0 {
		t.Errorf("expected empty config on runner error, got %+v", cfg)
	}
}
