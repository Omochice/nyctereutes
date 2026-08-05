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

// Splits a page into its frontmatter and the prose that follows. The fences are
// matched as whole lines, so a page is not rejected over the newline that
// happens to follow them or the carriage returns a checkout may add.
func split(page []byte) (frontmatter, prose string, err error) {
	lines := strings.Split(strings.ReplaceAll(string(page), "\r\n", "\n"), "\n")
	if lines[0] != fence {
		return "", "", errNoFrontmatter
	}
	end := slices.Index(lines[1:], fence)
	if end < 0 {
		return "", "", errUnclosedFrontmatter
	}
	frontmatter = strings.Join(lines[1:1+end], "\n")
	prose = strings.TrimLeft(strings.Join(lines[end+2:], "\n"), "\n")
	return frontmatter, prose, nil
}

// Reads the description a page declares in its frontmatter.
func describe(page []byte) (string, error) {
	frontmatter, _, err := split(page)
	if err != nil {
		return "", err
	}
	var declared struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &declared); err != nil {
		return "", fmt.Errorf("parse the frontmatter as YAML: %w", err)
	}
	if declared.Description == "" {
		return "", errNoDescription
	}
	return declared.Description, nil
}

// The page as a reader wants it: the frontmatter feeds `doc list` and says
// nothing a reader of the page needs. A page carrying none is returned whole,
// so a document stays readable even when its frontmatter is what is wrong.
func prose(page []byte) string {
	_, text, err := split(page)
	if err != nil {
		return string(page)
	}
	return text
}
