package doc

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/goccy/go-yaml"
)

// The extension every documentation page carries, and the suffix a page's name
// drops so that a name reads as a path into the tree.
const ext = ".md"

// The line that opens and closes a page's frontmatter.
const fence = "---\n"

var (
	errNoFrontmatter       = errors.New("no frontmatter")
	errUnclosedFrontmatter = errors.New("unclosed frontmatter")
)

// One page as `doc list` reports it.
type document struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Collects every page in fsys, which [fs.ReadDir] returns in name order. Only the
// top level is read, so a page's name is its file name and carries no path the
// reader would have to learn. The filesystem is a parameter rather than the
// embedded one so a test can present a tree the real documentation cannot.
func documents(fsys fs.FS) ([]document, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read the embedded documentation: %w", err)
	}
	docs := make([]document, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ext) {
			continue
		}
		page, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		description, err := describe(page)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		docs = append(docs, document{
			Name:        strings.TrimSuffix(name, ext),
			Description: description,
		})
	}
	return docs, nil
}

// Reads the description a page declares in its frontmatter. A page states it
// there rather than in its prose because the prose says what a command does,
// while a reader deciding which page to open needs to know when to open it.
func describe(page []byte) (string, error) {
	body, opened := strings.CutPrefix(string(page), fence)
	if !opened {
		return "", errNoFrontmatter
	}
	frontmatter, _, closed := strings.Cut(body, "\n"+fence)
	if !closed {
		return "", errUnclosedFrontmatter
	}
	var declared struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &declared); err != nil {
		return "", fmt.Errorf("parse the frontmatter as YAML: %w", err)
	}
	return declared.Description, nil
}
