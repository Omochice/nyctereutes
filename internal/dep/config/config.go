// Package config reads the dep.* settings from "glab config". Configuration is
// optional, so a missing key or an unavailable glab is treated as unset rather
// than as an error.
package config

import (
	"context"
	"strings"

	"github.com/Omochice/nyctereutes/internal/dep/textlist"
	"github.com/Omochice/nyctereutes/internal/glab"
)

// Holds the dep.* values read from glab config.
type Config struct {
	Repos    []string // dep.repo (comma-separated)
	Patterns []string // dep.patterns (newline-separated regex patterns)
	Author   string   // dep.author (default dependency bot username)
}

// Reads the dep.* settings through runner; an unset key or an unavailable glab
// yields zero values rather than an error.
func Load(ctx context.Context, runner glab.Runner) *Config {
	return &Config{
		Repos: textlist.SplitList(get(ctx, runner, "dep.repo")),
		// Patterns are regexes, which may contain commas, so they are listed one
		// per line rather than comma-separated.
		Patterns: textlist.SplitLines(get(ctx, runner, "dep.patterns")),
		Author:   strings.TrimSpace(get(ctx, runner, "dep.author")),
	}
}

// Reads a single glab config key, returning "" when unset or when glab is
// unavailable; config is optional, so such errors are not fatal.
func get(ctx context.Context, runner glab.Runner, key string) string {
	out, err := runner.Run(ctx, "config", "get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
