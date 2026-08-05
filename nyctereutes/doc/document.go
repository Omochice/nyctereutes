package doc

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/goccy/go-yaml"

	docfs "github.com/Omochice/nyctereutes/doc"
)

const (
	ext   = ".md"
	dir   = "cmd"
	fence = "---\n"
)

var (
	errNoFrontmatter       = errors.New("no frontmatter")
	errUnclosedFrontmatter = errors.New("unclosed frontmatter")
)

// One page as `doc list` reports it.
type document struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// The embedded pages, with the directory holding them as the root.
func pages() (fs.FS, error) {
	rooted, err := fs.Sub(docfs.FS, dir)
	if err != nil {
		return nil, fmt.Errorf("open the embedded documentation: %w", err)
	}
	return rooted, nil
}

// Collects the pages at the root of fsys, in name order.
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

// Reads the description a page declares in its frontmatter.
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
