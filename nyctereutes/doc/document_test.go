package doc

import (
	"slices"
	"strings"
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

// The order pins what `doc list` promises its reader. It holds today because
// the walk is lexical, and the test keeps it holding if the collector is ever
// rewritten around a map, whose range order is deliberately unstable.
func TestDocumentsAreOrderedByName(t *testing.T) {
	described := page("---\ndescription: A page.\n---\n\n# page\n")
	fsys := fstest.MapFS{
		"cmd/zeta.md":  described,
		"cmd/alpha.md": described,
		"top.md":       described,
	}

	docs, err := documents(fsys)
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	names := make([]string, 0, len(docs))
	for _, doc := range docs {
		names = append(names, doc.Name)
	}
	want := []string{"cmd/alpha", "cmd/zeta", "top"}
	if !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestDocumentsRejectAPageThatDeclaresNoDescription(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
	}{
		{"no frontmatter at all", "# bare\n"},
		{"frontmatter never closed", "---\ndescription: A page.\n\n# bare\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fsys := fstest.MapFS{"cmd/bare.md": page(testCase.body)}

			_, err := documents(fsys)
			if err == nil {
				t.Fatal("documents succeeded, want a failure naming the page")
			}
			if !strings.Contains(err.Error(), "cmd/bare.md") {
				t.Errorf("error does not name the page: %v", err)
			}
		})
	}
}
