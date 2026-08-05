package doc

import (
	"testing"
	"testing/fstest"
)

// A page as the collector sees it, written out here so the tests can state a
// tree the shipped documentation cannot contain.
func page(body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(body)}
}

func TestDocumentsReadTheDescriptionFromFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		"cmd/thing.md": page("---\ndescription: What thing does, and when to read this.\n---\n\n# thing\n"),
	}

	docs, err := documents(fsys)
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want 1", len(docs))
	}
	if want := "cmd/thing"; docs[0].Name != want {
		t.Errorf("name = %q, want %q", docs[0].Name, want)
	}
	if want := "What thing does, and when to read this."; docs[0].Description != want {
		t.Errorf("description = %q, want %q", docs[0].Description, want)
	}
}
