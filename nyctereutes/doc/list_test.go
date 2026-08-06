package doc_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Omochice/nyctereutes/cli"
	docfs "github.com/Omochice/nyctereutes/doc"
	"github.com/Omochice/nyctereutes/nyctereutes/doc"
)

type listOutput struct {
	Results []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"results"`
	Help string `json:"help"`
}

func decodeList(t *testing.T, stdout string) listOutput {
	t.Helper()
	var got listOutput
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout as JSON: %v\n%s", err, stdout)
	}
	return got
}

func TestDocListWritesResultsAndHelpAsJSON(t *testing.T) {
	exit, stdout, stderr := run("doc", "list")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	got := decodeList(t, stdout)
	if len(got.Results) == 0 {
		t.Error("results is empty, want an entry for every embedded document")
	}
	if got.Help == "" {
		t.Error("help is empty, want the instruction to run doc show")
	}
}

func TestDocListWritesTheHelpTextLiterally(t *testing.T) {
	exit, stdout, stderr := run("doc", "list")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	if want := "doc show <name>"; !strings.Contains(stdout, want) {
		t.Errorf("stdout does not carry %q literally:\n%s", want, stdout)
	}
}

func TestDocListDescribesEveryShippedDocument(t *testing.T) {
	entries, err := fs.ReadDir(docfs.Pages(), ".")
	if err != nil {
		t.Fatalf("read the embedded documentation: %v", err)
	}

	exit, stdout, stderr := run("doc", "list")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	results := decodeList(t, stdout).Results
	if len(results) != len(entries) {
		t.Errorf("got %d results for %d embedded pages", len(results), len(entries))
	}
	for _, result := range results {
		if result.Description == "" {
			t.Errorf("%s has no description, want the one declared in its frontmatter", result.Name)
		}
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want no page reported as unreadable", stderr)
	}
}

func TestDocListReportsAMalformedPageWithoutWithholdingTheRest(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	command := doc.New(&cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: stdout,
		Stderr: stderr,
	})
	doc.SetPages(command, fstest.MapFS{
		"good.md": &fstest.MapFile{Data: []byte("---\ndescription: A page.\n---\n\n# good\n")},
		"bad.md":  &fstest.MapFile{Data: []byte("# bad\n")},
	})

	if err := command.List.Execute(nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	results := decodeList(t, stdout.String()).Results
	if len(results) != 1 || results[0].Name != "good" {
		t.Errorf("results = %v, want only the page that could be read", results)
	}
	if !strings.Contains(stderr.String(), "bad.md") {
		t.Errorf("stderr does not name the malformed page\n%s", stderr)
	}
}
