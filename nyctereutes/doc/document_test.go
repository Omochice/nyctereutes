package doc

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func page(body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(body)}
}

func TestDocumentsReadTheDescriptionFromFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		"thing.md": page("---\ndescription: What thing does, and when to read this.\n---\n\n# thing\n"),
	}

	docs, problems, err := documents(fsys)
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	if len(problems) != 0 {
		t.Errorf("problems = %v, want none", problems)
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

func TestDocumentsIgnoreAnythingBelowTheTopLevel(t *testing.T) {
	described := page("---\ndescription: A page.\n---\n\n# page\n")
	fsys := fstest.MapFS{
		"thing.md":        described,
		"nested/other.md": described,
	}

	docs, _, err := documents(fsys)
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	if len(docs) != 1 || docs[0].Name != "thing" {
		t.Errorf("documents = %v, want only the top-level thing", docs)
	}
}

func TestDocumentsAreOrderedByName(t *testing.T) {
	described := page("---\ndescription: A page.\n---\n\n# page\n")
	fsys := fstest.MapFS{
		"zeta.md":  described,
		"alpha.md": described,
		"mid.md":   described,
	}

	docs, _, err := documents(fsys)
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

func TestDocumentsKeepThePagesThatCanBeRead(t *testing.T) {
	fsys := fstest.MapFS{
		"good.md": page("---\ndescription: A page.\n---\n\n# good\n"),
		"bad.md":  page("# bad\n"),
	}

	docs, problems, err := documents(fsys)
	if err != nil {
		t.Fatalf("documents: %v", err)
	}
	if len(docs) != 1 || docs[0].Name != "good" {
		t.Errorf("documents = %v, want only the page that could be read", docs)
	}
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want the unreadable page reported once", len(problems))
	}
	if !strings.Contains(problems[0].Error(), "bad.md") {
		t.Errorf("problem does not name the page: %v", problems[0])
	}
}

func TestDescribeAcceptsEveryClosedFrontmatter(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
		want string
	}{
		{"declared description", "---\ndescription: A page.\n---\n\n# p\n", "A page."},
		{"no newline after the closing fence", "---\ndescription: A page.\n---", "A page."},
		{"carriage returns", "---\r\ndescription: A page.\r\n---\r\n\r\n# p\r\n", "A page."},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := describe([]byte(testCase.body))
			if err != nil {
				t.Fatalf("describe: %v", err)
			}
			if got != testCase.want {
				t.Errorf("description = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestDescribeRejectsAPageWithoutADescription(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
	}{
		{"no frontmatter at all", "# bare\n"},
		{"frontmatter never closed", "---\ndescription: A page.\n\n# bare\n"},
		{"empty frontmatter", "---\n---\n\n# bare\n"},
		{"another key but no description", "---\ntitle: bare\n---\n\n# bare\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := describe([]byte(testCase.body)); err == nil {
				t.Error("describe succeeded, want a failure")
			}
		})
	}
}

var errUnreadableTree = errors.New("unreadable tree")

// A filesystem that cannot be listed at all, which is a different failure from
// a name no page carries.
type unreadableFS struct{}

func (unreadableFS) Open(string) (fs.File, error) {
	return nil, errUnreadableTree
}

func TestNotFoundReportsAnUnreadableTreeAsItself(t *testing.T) {
	err := notFound(unreadableFS{}, "nope")

	if err == nil {
		t.Fatal("notFound succeeded, want the failure to read the tree")
	}
	if errors.Is(err, errNoSuchDocument) {
		t.Errorf("a tree that cannot be read is reported as an absent document: %v", err)
	}
	if !errors.Is(err, errUnreadableTree) {
		t.Errorf("error does not carry the reason the tree could not be read: %v", err)
	}
}
