package doc

import (
	"fmt"
	"io/fs"
	"strings"
)

// The extension every documentation page carries, and the suffix a page's name
// drops so that a name reads as a path into the tree.
const ext = ".md"

// One page as `doc list` reports it.
type document struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Collects every page in fsys. The filesystem is a parameter rather than the
// embedded one so a test can present a tree the real documentation cannot.
func documents(fsys fs.FS) ([]document, error) {
	var docs []document
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ext) {
			return nil
		}
		docs = append(docs, document{Name: strings.TrimSuffix(path, ext)})
		return nil
	}
	if err := fs.WalkDir(fsys, ".", walk); err != nil {
		return nil, fmt.Errorf("walk the embedded documentation: %w", err)
	}
	return docs, nil
}
