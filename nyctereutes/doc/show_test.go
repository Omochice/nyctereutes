package doc_test

import (
	"io/fs"
	"strings"
	"testing"

	docfs "github.com/Omochice/nyctereutes/doc"
)

func TestDocShowWritesTheDocumentUnchanged(t *testing.T) {
	want, err := fs.ReadFile(docfs.Pages(), "dep.md")
	if err != nil {
		t.Fatalf("read the embedded page the test compares against: %v", err)
	}

	exit, stdout, stderr := run("doc", "show", "dep")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", exit, stderr)
	}
	if stdout != string(want) {
		t.Errorf("stdout = %q, want the page unchanged %q", stdout, want)
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
