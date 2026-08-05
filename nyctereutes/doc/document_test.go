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
		"thing.md": page("---\ndescription: What thing does, and when to read this.\n---\n\n# thing\n"),
	}

	docs, err := documents(fsys)
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want 1", len(docs))
	}
	if want := "thing"; docs[0].Name != want {
		t.Errorf("name = %q, want %q", docs[0].Name, want)
	}
	if want := "What thing does, and when to read this."; docs[0].Description != want {
		t.Errorf("description = %q, want %q", docs[0].Description, want)
	}
}

// A name is what a reader types, so it says nothing about where the file sits.
// Keeping the tree flat is what makes that possible, and a nested page would
// have to be named around the directory holding it.
func TestDocumentsIgnoreAnythingBelowTheTopLevel(t *testing.T) {
	described := page("---\ndescription: A page.\n---\n\n# page\n")
	fsys := fstest.MapFS{
		"thing.md":     described,
		"cmd/other.md": described,
	}

	docs, err := documents(fsys)
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	if len(docs) != 1 || docs[0].Name != "thing" {
		t.Errorf("documents = %v, want only the top-level thing", docs)
	}
}

// The order pins what `doc list` promises its reader. It holds today because
// the directory read is sorted, and the test keeps it holding if the collector
// is ever rewritten around a map, whose range order is deliberately unstable.
func TestDocumentsAreOrderedByName(t *testing.T) {
	described := page("---\ndescription: A page.\n---\n\n# page\n")
	fsys := fstest.MapFS{
		"zeta.md":  described,
		"alpha.md": described,
		"mid.md":   described,
	}

	docs, err := documents(fsys)
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	names := make([]string, 0, len(docs))
	for _, doc := range docs {
		names = append(names, doc.Name)
	}
	want := []string{"alpha", "mid", "zeta"}
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
			fsys := fstest.MapFS{"bare.md": page(testCase.body)}

			_, err := documents(fsys)
			if err == nil {
				t.Fatal("documents succeeded, want a failure naming the page")
			}
			if !strings.Contains(err.Error(), "bare.md") {
				t.Errorf("error does not name the page: %v", err)
			}
		})
	}
}
