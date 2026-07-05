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
	ErrNotFound   = errors.New("glab: not found (404)")
	ErrForbidden  = errors.New("glab: forbidden (403)")
	ErrValidation = errors.New("glab: validation failed (400/422)")
)

// Detection keys on the "HTTP <code>" token glab always appends to a failed
// run's stderr (for example "glab: 404 Project Not Found (HTTP 404)", or a bare
// "glab: HTTP 400" when the body carries no message). Matching that token
// rather than a bare status number avoids classifying an unrelated error whose
// text merely contains the digits (a "project-404" name, a retry count), and
// the descriptive phrase is unreliable: a 400 emits none on stderr. An
// unmatched status loses no information, since the raw stderr stays in the
// wrapping error.
func classify(stderr string) error {
	switch {
	case strings.Contains(stderr, "HTTP 404"):
		return ErrNotFound
	case strings.Contains(stderr, "HTTP 403"):
		return ErrForbidden
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
