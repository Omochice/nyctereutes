package manifest

import (
	"strings"
	"testing"
)

const validDoc = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  visibility: private
  topics: []
`

// joinDocs assembles a multi-document YAML stream the way import emits it:
// documents separated by a bare "---" line.
func joinDocs(docs ...string) []byte {
	return []byte(strings.Join(docs, "---\n"))
}

func TestParseReadsSingleDocument(t *testing.T) {
	repos, errs := Parse([]byte(validDoc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if len(repos) != 1 {
		t.Fatalf("parsed %d documents, want 1", len(repos))
	}
	if repos[0].Metadata.Owner != "group" || repos[0].Metadata.Name != "proj" {
		t.Errorf("metadata = %+v, want owner=group name=proj", repos[0].Metadata)
	}
}

// Import emits one document per project separated by "---", so validate must
// read the stream back in full.
func TestParseReadsAllDocuments(t *testing.T) {
	second := strings.ReplaceAll(validDoc, "name: proj", "name: other")
	repos, errs := Parse(joinDocs(validDoc, second))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if len(repos) != 2 {
		t.Fatalf("parsed %d documents, want 2", len(repos))
	}
	if repos[1].Metadata.Name != "other" {
		t.Errorf("second document name = %q, want %q", repos[1].Metadata.Name, "other")
	}
}

// Blank documents between separators carry nothing to validate, and an empty
// file is simply zero documents, not an error.
func TestParseSkipsEmptyDocuments(t *testing.T) {
	repos, errs := Parse(joinDocs("", validDoc, "\n"))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if len(repos) != 1 {
		t.Errorf("parsed %d documents, want 1 with empty documents skipped", len(repos))
	}

	repos, errs = Parse(nil)
	if len(repos) != 0 || len(errs) != 0 {
		t.Errorf("empty input parsed to %d documents and %v, want none", len(repos), errs)
	}
}

// The schema structs are the single source of truth, so a key they do not
// declare is a validation error, not silently ignored data.
func TestParseRejectsUnknownKey(t *testing.T) {
	doc := validDoc + "  unknown_key: x\n"
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one unknown-key error", errs)
	}
	if !strings.Contains(errs[0].Error(), "unknown_key") {
		t.Errorf("error %q does not name the unknown key", errs[0])
	}
}

func TestParseRejectsUnknownKind(t *testing.T) {
	doc := strings.ReplaceAll(validDoc, "kind: Repository", "kind: FileSet")
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one unknown-kind error", errs)
	}
	if !strings.Contains(errs[0].Error(), "FileSet") {
		t.Errorf("error %q does not name the unknown kind", errs[0])
	}
}

func TestParseRejectsUnknownAPIVersion(t *testing.T) {
	doc := strings.ReplaceAll(validDoc, "apiVersion: nyctereutes/v1", "apiVersion: gh-infra/v1")
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one apiVersion error", errs)
	}
	if !strings.Contains(errs[0].Error(), "gh-infra/v1") {
		t.Errorf("error %q does not name the unsupported apiVersion", errs[0])
	}
}

// The metadata fields address the GitLab project, so a document without them
// cannot be applied and must not validate.
func TestParseRequiresMetadataFields(t *testing.T) {
	cases := []struct{ name, drop, wantField string }{
		{"missing_name", "  name: proj\n", "metadata.name"},
		{"missing_owner", "  owner: group\n", "metadata.owner"},
	}
	for _, attr := range cases {
		t.Run(attr.name, func(t *testing.T) {
			doc := strings.ReplaceAll(validDoc, attr.drop, "")
			_, errs := Parse([]byte(doc))
			if len(errs) != 1 {
				t.Fatalf("errs = %v, want exactly one required-field error", errs)
			}
			if !strings.Contains(errs[0].Error(), attr.wantField) {
				t.Errorf("error %q does not name %s", errs[0], attr.wantField)
			}
		})
	}
}

// A hand-written manifest may omit spec fields entirely; only metadata is
// required.
func TestParseAllowsMinimalSpec(t *testing.T) {
	doc := "apiVersion: nyctereutes/v1\nkind: Repository\nmetadata:\n  name: proj\n  owner: group\n"
	repos, errs := Parse([]byte(doc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want a minimal document to validate", errs)
	}
	if len(repos) != 1 {
		t.Errorf("parsed %d documents, want 1", len(repos))
	}
}

// One broken document must not hide problems in — or the validity of — the
// documents after it, and its error must say which document it is.
func TestParseContinuesPastInvalidDocument(t *testing.T) {
	bad := strings.ReplaceAll(validDoc, "kind: Repository", "kind: Nonsense")
	repos, errs := Parse(joinDocs(validDoc, bad, validDoc))
	if len(repos) != 2 {
		t.Errorf("parsed %d documents, want the 2 valid ones", len(repos))
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one error for the broken document", errs)
	}
	if !strings.Contains(errs[0].Error(), "document 2") {
		t.Errorf("error %q does not carry the document position", errs[0])
	}
}
