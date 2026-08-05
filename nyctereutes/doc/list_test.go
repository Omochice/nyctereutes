package doc_test

import (
	"encoding/json"
	"testing"
)

// The shape `doc list` promises its reader: an entry per document plus the
// instruction for reading one.
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

// Guards the shipped pages rather than the collector: a page added without a
// description would still be listed, and a reader choosing between pages would
// have nothing to choose on.
func TestDocListDescribesEveryShippedDocument(t *testing.T) {
	exit, stdout, stderr := run("doc", "list")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	for _, result := range decodeList(t, stdout).Results {
		if result.Description == "" {
			t.Errorf("%s has no description, want the one declared in its frontmatter", result.Name)
		}
	}
}
