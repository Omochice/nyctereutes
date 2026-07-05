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

// Detection is by substring because glab surfaces the HTTP status and phrase
// verbatim; the raw stderr stays in the wrapping error, so an unmatched status
// loses no information.
func classify(stderr string) error {
	switch {
	case strings.Contains(stderr, "404") || strings.Contains(stderr, "Not Found"):
		return ErrNotFound
	case strings.Contains(stderr, "403") || strings.Contains(stderr, "Forbidden"):
		return ErrForbidden
	case strings.Contains(stderr, "400") || strings.Contains(stderr, "422") ||
		strings.Contains(stderr, "Unprocessable") || strings.Contains(stderr, "validation"):
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
