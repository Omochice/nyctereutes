package doc_test

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Omochice/nyctereutes/cli"
	docfs "github.com/Omochice/nyctereutes/doc"
	"github.com/Omochice/nyctereutes/nyctereutes/doc"
)

func TestDocShowWritesTheDocumentWithoutItsFrontmatter(t *testing.T) {
	page, err := fs.ReadFile(docfs.Pages(), "dep.md")
	if err != nil {
		t.Fatalf("read the embedded page the test compares against: %v", err)
	}
	_, afterOpeningFence, _ := strings.Cut(string(page), "---\n")
	_, want, found := strings.Cut(afterOpeningFence, "---\n")
	if !found {
		t.Fatalf("the page the test compares against carries no frontmatter:\n%s", page)
	}
	want = strings.TrimLeft(want, "\n")

	exit, stdout, stderr := run("doc", "show", "dep")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	if stdout != want {
		t.Errorf("stdout = %q, want the page without its frontmatter %q", stdout, want)
	}
}

func TestDocShowReportsAnUnknownNameWithTheAvailableOnes(t *testing.T) {
	exit, stdout, stderr := run("doc", "show", "nope")

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 for a name no page carries", exit)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing written for a failed lookup", stdout)
	}
	for _, want := range []string{"nope", "dep", "infra"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q\n%s", want, stderr)
		}
	}
}

func TestDocShowRequiresAName(t *testing.T) {
	exit, stdout, stderr := run("doc", "show")

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 when no name is given", exit)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing written without a name", stdout)
	}
	if stderr == "" {
		t.Error("stderr is empty, want the missing name reported")
	}
}

func TestDocShowRejectsASecondName(t *testing.T) {
	exit, stdout, stderr := run("doc", "show", "dep", "infra")

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 when more than one name is given", exit)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want no page written for a rejected invocation", stdout)
	}
	if !strings.Contains(stderr, "infra") {
		t.Errorf("stderr does not name the argument that was rejected\n%s", stderr)
	}
}

func TestDocShowReportsAnUnreadableNameAsItself(t *testing.T) {
	exit, _, stderr := run("doc", "show", "../secret")

	if exit != 1 {
		t.Fatalf("exit = %d, want 1 for a name that is not a valid path", exit)
	}
	if strings.Contains(stderr, "no such document") {
		t.Errorf("stderr reports absence for a name that failed to resolve\n%s", stderr)
	}
}

func TestDocShowOffersANameItCanStillRead(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	command := doc.New(&cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: stdout,
		Stderr: stderr,
	})
	doc.SetPages(command, fstest.MapFS{
		"described.md":   &fstest.MapFile{Data: []byte("---\ndescription: A page.\n---\n\n# described\n")},
		"undescribed.md": &fstest.MapFile{Data: []byte("# undescribed\n")},
	})

	err := command.Show.Execute([]string{"nope"})

	if err == nil {
		t.Fatal("Execute succeeded, want a failure naming the pages that exist")
	}
	for _, want := range []string{"described", "undescribed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not offer %q, which doc show can read\n%v", want, err)
		}
	}
}
