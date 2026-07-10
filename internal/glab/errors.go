package glab

import (
	"errors"
	"fmt"
	"strings"
)

// The GitLab API failures a caller branches on. glabError wraps a failed run
// with one of these so consumers use [errors.Is] rather than matching status
// text.
var (
	ErrNotFound     = errors.New("glab: not found (404)")
	ErrForbidden    = errors.New("glab: forbidden (403)")
	ErrUnauthorized = errors.New("glab: unauthorized (401)")
	ErrValidation   = errors.New("glab: validation failed (400/422)")
)

// Keys on the "HTTP <code>" token glab appends to a failed run's stderr, not a
// bare status number: an unrelated error that merely mentions the digits (a
// "project-404" name, a retry count) must not classify, and a 400 emits no
// phrase to match on instead. The mr-family subcommands print client-go's
// "<method> <url>: <code> <message>" format with no HTTP token, so a 401 is
// additionally keyed on its status text, which spaces keep out of URLs and
// project paths.
func classify(stderr string) error {
	switch {
	case strings.Contains(stderr, "HTTP 404"):
		return ErrNotFound
	case strings.Contains(stderr, "HTTP 403"):
		return ErrForbidden
	case strings.Contains(stderr, "HTTP 401") || strings.Contains(stderr, "401 Unauthorized"):
		return ErrUnauthorized
	case strings.Contains(stderr, "HTTP 400") || strings.Contains(stderr, "HTTP 422"):
		return ErrValidation
	}
	return nil
}

// Wraps a failed run's command and stderr with a classification sentinel when
// one applies, so callers branch with [errors.Is] while the raw stderr stays
// readable.
func glabError(args []string, runErr error, stderr string) error {
	base := fmt.Errorf("glab %s: %w\n%s", strings.Join(args, " "), runErr, strings.TrimSpace(stderr))
	if sentinel := classify(stderr); sentinel != nil {
		return fmt.Errorf("%w: %w", sentinel, base)
	}
	return base
}
