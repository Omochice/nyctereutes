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
		{"not found by status", "404 Project Not Found", ErrNotFound},
		{"not found by phrase", `{"message":"Not Found"}`, ErrNotFound},
		{"forbidden", "403 Forbidden", ErrForbidden},
		{"validation 422", "422 Unprocessable Entity", ErrValidation},
		{"validation 400", "400 Bad Request", ErrValidation},
		{"validation phrase", "validation failed: name is taken", ErrValidation},
		{"unclassified", "500 Internal Server Error", nil},
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

	err := glabError([]string{"api", "projects/x"}, base, "404 Project Not Found")

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
