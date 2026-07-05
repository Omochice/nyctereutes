package glab

import (
	"errors"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		stderr string
		want   error
	}{
		// The stderr strings mirror glab's real output, which always ends in an
		// "HTTP <code>" token; a 400 carries no message on stderr.
		{"not found", "glab: 404 Project Not Found (HTTP 404)", ErrNotFound},
		{"forbidden", "glab: 403 Forbidden (HTTP 403)", ErrForbidden},
		{"validation 400 bare", "glab: HTTP 400", ErrValidation},
		{"validation 422", "glab: 422 Unprocessable Entity (HTTP 422)", ErrValidation},
		{"unclassified 500", "glab: 500 Internal Server Error (HTTP 500)", nil},
		// A digit run that is not the HTTP status token must not classify: an
		// unrelated failure whose text merely mentions 404 stays unclassified.
		{"unrelated 404 digits", "glab: request failed after 404 retries", nil},
		// A validation error naming a project that contains 404 must classify by
		// its HTTP 400 token, not be shadowed into ErrNotFound by the digits.
		{"validation naming a 404 project", "glab: 400 name project-404 is taken (HTTP 400)", ErrValidation},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := classify(testCase.stderr); !errors.Is(got, testCase.want) {
				t.Errorf("classify(%q) = %v, want %v", testCase.stderr, got, testCase.want)
			}
		})
	}
}

func TestGlabErrorWrapsSentinelAndKeepsStderr(t *testing.T) {
	base := errors.New("exit status 1")

	err := glabError([]string{"api", "projects/x"}, base, "glab: 404 Project Not Found (HTTP 404)")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false for a 404 stderr")
	}
	if !strings.Contains(err.Error(), "404 Project Not Found") {
		t.Errorf("error should carry the raw stderr, got %q", err.Error())
	}
}

func TestGlabErrorLeavesUnclassifiedAlone(t *testing.T) {
	base := errors.New("exit status 1")

	err := glabError([]string{"api", "projects/x"}, base, "500 Internal Server Error")

	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrValidation) {
		t.Errorf("a 500 should carry no classification sentinel, got %v", err)
	}
	if !strings.Contains(err.Error(), "500 Internal Server Error") {
		t.Errorf("error should carry the raw stderr, got %q", err.Error())
	}
}
