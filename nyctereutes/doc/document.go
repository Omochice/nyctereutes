package doc

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	ext   = ".md"
	fence = "---"
)

var (
	errNoFrontmatter       = errors.New("no frontmatter")
	errUnclosedFrontmatter = errors.New("unclosed frontmatter")
	errNoDescription       = errors.New("frontmatter declares no description")
)

// One page as `doc list` reports it.
type document struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Collects the pages at the root of fsys, in name order, along with one problem
// per page that could not be described. A page nobody can read is reported and
// left out rather than taking the readable pages down with it.
func documents(fsys fs.FS) ([]document, []error, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("read the embedded documentation: %w", err)
	}
	docs := make([]document, 0, len(entries))
	var problems []error
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ext) {
			continue
		}
		page, err := fs.ReadFile(fsys, name)
		if err != nil {
			problems = append(problems, fmt.Errorf("read %s: %w", name, err))
			continue
		}
		description, err := describe(page)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
			continue
		}
		docs = append(docs, document{
			Name:        strings.TrimSuffix(name, ext),
			Description: description,
		})
	}
	return docs, problems, nil
}

// Reads the description a page declares in its frontmatter. The fences are
// matched as whole lines, so a page is not rejected over the newline that
// happens to follow them or the carriage returns a checkout may add.
func describe(page []byte) (string, error) {
	lines := strings.Split(strings.ReplaceAll(string(page), "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != fence {
		return "", errNoFrontmatter
	}
	end := slices.Index(lines[1:], fence)
	if end < 0 {
		return "", errUnclosedFrontmatter
	}
	var declared struct {
		Description string `yaml:"description"`
	}
	frontmatter := strings.Join(lines[1:1+end], "\n")
	if err := yaml.Unmarshal([]byte(frontmatter), &declared); err != nil {
		return "", fmt.Errorf("parse the frontmatter as YAML: %w", err)
	}
	if declared.Description == "" {
		return "", errNoDescription
	}
	return declared.Description, nil
}
