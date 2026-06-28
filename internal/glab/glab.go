// Package glab invokes the glab CLI. The glab CLI owns the GitLab credentials;
// this tool never stores a token itself. The Runner interface lets callers
// inject a fake in tests instead of shelling out to a real glab.
package glab

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes the glab CLI with the given arguments and returns its stdout.
type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

// RunnerFunc adapts an ordinary function to a Runner, mainly so tests can script
// responses with a closure instead of declaring a fake type.
type RunnerFunc func(ctx context.Context, args ...string) ([]byte, error)

func (f RunnerFunc) Run(ctx context.Context, args ...string) ([]byte, error) {
	return f(ctx, args...)
}

// ExecRunner is the production Runner backed by the real glab executable.
type ExecRunner struct{}

// Run executes "glab <args...>". On failure the error includes glab's stderr so
// the underlying cause (for example a missing login) is surfaced verbatim.
func (ExecRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	// glab is a fixed trusted binary; passing dynamic args to it is this
	// package's entire purpose.
	cmd := exec.CommandContext(ctx, "glab", args...) //nolint:gosec // G204: args are intended dynamic glab arguments
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("glab %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
