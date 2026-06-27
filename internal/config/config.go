// Package config reads the dep.* settings from "glab config". Configuration is
// optional, so a missing key or an unavailable glab is treated as unset rather
// than as an error.
package config

import (
	"context"
	"strings"

	"github.com/Omochice/nyctereutes/internal/glab"
	"github.com/Omochice/nyctereutes/internal/strutil"
)

// Config holds the dep.* values read from glab config.
type Config struct {
	Repos    []string // dep.repo (comma-separated)
	Patterns []string // dep.patterns (comma-separated regex patterns)
	Author   string   // dep.author (default dependency bot username)
}

func Load(ctx context.Context, runner glab.Runner) *Config {
	return &Config{
		Repos:    strutil.SplitList(get(ctx, runner, "dep.repo")),
		Patterns: strutil.SplitList(get(ctx, runner, "dep.patterns")),
		Author:   strings.TrimSpace(get(ctx, runner, "dep.author")),
	}
}

// get reads a single glab config key, returning "" when unset or when glab is
// unavailable; config is optional, so such errors are not fatal.
func get(ctx context.Context, runner glab.Runner, key string) string {
	out, err := runner.Run(ctx, "config", "get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
