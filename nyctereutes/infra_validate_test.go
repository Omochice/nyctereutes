package nyctereutes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifest = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  visibility: private
  topics: []
`

// writeManifest writes content into dir under name and returns the full path.
func writeManifest(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func brokenManifest() string {
	return strings.ReplaceAll(validManifest, "kind: Repository", "kind: Nonsense")
}

func TestInfraValidateRequiresPath(t *testing.T) {
	exit, _, _ := runDep(&fakeInfraGlab{}, "infra", "validate")
	if exit != 1 {
		t.Errorf("exit = %d, want 1 when no path is given", exit)
	}
}

func TestInfraValidateSummarizesValidManifests(t *testing.T) {
	stream := validManifest + "---\n" + strings.ReplaceAll(validManifest, "name: proj", "name: other")
	path := writeManifest(t, t.TempDir(), "a.yaml", stream)

	exit, stdout, _ := runDep(&fakeInfraGlab{}, "infra", "validate", path)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0 for a valid manifest", exit)
	}
	for _, want := range []string{"2 repositories", "group/proj", "group/other"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

func TestInfraValidateReportsDocumentPosition(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "a.yaml", validManifest+"---\n"+brokenManifest())

	exit, _, stderr := runDep(&fakeInfraGlab{}, "infra", "validate", path)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a document is invalid", exit)
	}
	for _, want := range []string{"a.yaml", "document 2"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q\n%s", want, stderr)
		}
	}
}

// Problems in an early file must not stop later files from being checked, so
// one run reports everything there is to fix.
func TestInfraValidateContinuesPastBrokenFile(t *testing.T) {
	dir := t.TempDir()
	first := writeManifest(t, dir, "a.yaml", brokenManifest())
	second := writeManifest(t, dir, "b.yaml", brokenManifest())

	exit, _, stderr := runDep(&fakeInfraGlab{}, "infra", "validate", first, second)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when documents are invalid", exit)
	}
	for _, want := range []string{"a.yaml", "b.yaml"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q, later files must still be validated\n%s", want, stderr)
		}
	}
}

// A directory argument covers its .yaml/.yml entries one level deep; other
// extensions and subdirectories are not validation targets.
func TestInfraValidateWalksDirectoryNonRecursively(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "a.yaml", validManifest)
	writeManifest(t, dir, "b.yml", strings.ReplaceAll(validManifest, "name: proj", "name: other"))
	writeManifest(t, dir, "c.txt", "not a manifest")
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeManifest(t, sub, "d.yaml", brokenManifest())

	exit, stdout, stderr := runDep(&fakeInfraGlab{}, "infra", "validate", dir)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0: c.txt and sub/d.yaml are not targets\n%s", exit, stderr)
	}
	if !strings.Contains(stdout, "2 repositories") {
		t.Errorf("stdout missing %q\n%s", "2 repositories", stdout)
	}
}

func TestInfraValidateReportsMissingPath(t *testing.T) {
	dir := t.TempDir()
	good := writeManifest(t, dir, "a.yaml", validManifest)

	exit, _, stderr := runDep(&fakeInfraGlab{}, "infra", "validate", filepath.Join(dir, "nope.yaml"), good)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a path does not exist", exit)
	}
	if !strings.Contains(stderr, "nope.yaml") {
		t.Errorf("stderr missing the unreadable path\n%s", stderr)
	}
}

func TestInfraValidateAcceptsEmptyFile(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "empty.yaml", "")

	exit, stdout, _ := runDep(&fakeInfraGlab{}, "infra", "validate", path)

	if exit != 0 {
		t.Errorf("exit = %d, want 0 for an empty file", exit)
	}
	if !strings.Contains(stdout, "0 repositories") {
		t.Errorf("stdout missing %q\n%s", "0 repositories", stdout)
	}
}
