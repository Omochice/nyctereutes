package glab

import (
	"errors"
	"fmt"
	"strings"
)

// Classification sentinels for the GitLab API errors callers branch on. A
// failed run wraps its error with one of these when the stderr identifies the
// status, so consumers use [errors.Is] rather than matching status text.
var (
	ErrNotFound   = errors.New("glab: not found (404)")
	ErrForbidden  = errors.New("glab: forbidden (403)")
	ErrValidation = errors.New("glab: validation failed (400/422)")
)

// classify inspects glab's stderr for the HTTP status or phrase of a GitLab API
// error and returns the matching sentinel, or nil when none applies. Detection
// is by substring because glab surfaces the status and phrase verbatim and the
// raw stderr is kept in the wrapping error, so a miss loses no information.
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

// glabError builds the error a failed glab run returns: the command and its
// stderr, wrapped with a classification sentinel when one applies so callers
// can branch with [errors.Is].
func glabError(args []string, runErr error, stderr string) error {
	base := fmt.Errorf("glab %s: %w\n%s", strings.Join(args, " "), runErr, strings.TrimSpace(stderr))
	if sentinel := classify(stderr); sentinel != nil {
		return fmt.Errorf("%w: %w", sentinel, base)
	}
	return base
}
