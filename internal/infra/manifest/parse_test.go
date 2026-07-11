package manifest

import (
	"errors"
	"fmt"
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

// GitLab refuses to publish a project to the CI/CD Catalog without a
// description, so a manifest asking for the catalog must carry one; both an
// omitted and an empty description fail.
func TestParseRequiresDescriptionForCICatalog(t *testing.T) {
	cases := []struct{ name, description string }{
		{"description_omitted", ""},
		{"description_empty", "  description: \"\"\n"},
	}
	for _, attr := range cases {
		t.Run(attr.name, func(t *testing.T) {
			doc := validDoc + "  ci_catalog: true\n" + attr.description
			_, errs := Parse([]byte(doc))
			if len(errs) != 1 {
				t.Fatalf("errs = %v, want exactly one required-field error", errs)
			}
			if !strings.Contains(errs[0].Error(), "spec.description") {
				t.Errorf("error %q does not name the missing description", errs[0])
			}
		})
	}
}

// A catalog project that does carry a description validates: the prerequisite
// is met, so the manifest can apply.
func TestParseAllowsCICatalogWithDescription(t *testing.T) {
	doc := validDoc + "  ci_catalog: true\n  description: a reusable component\n"
	repos, errs := Parse([]byte(doc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want a described catalog project to validate", errs)
	}
	if len(repos) != 1 {
		t.Errorf("parsed %d documents, want 1", len(repos))
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

// A document that is not even parseable YAML must not take the rest of the
// stream down with it either.
func TestParseContinuesPastSyntaxError(t *testing.T) {
	repos, errs := Parse(joinDocs(validDoc, ": : :\n", validDoc))
	if len(repos) != 2 {
		t.Errorf("parsed %d documents, want the 2 valid ones", len(repos))
	}
	if len(errs) != 1 {
		t.Errorf("errs = %v, want exactly one error for the malformed document", errs)
	}
}

// A UTF-8 BOM (written by some Windows editors, permitted at stream start by
// the YAML spec) must not glue itself onto the first key and surface as a
// baffling unsupported-apiVersion error.
func TestParseStripsLeadingBOM(t *testing.T) {
	repos, errs := Parse([]byte("\ufeff" + validDoc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want a BOM-prefixed manifest to validate", errs)
	}
	if len(repos) != 1 {
		t.Errorf("parsed %d documents, want 1", len(repos))
	}
}

// A file holding only a document start marker is semantically empty, the
// same as an empty file; it must not be reported as an unknown kind.
func TestParseAcceptsMarkerOnlyStream(t *testing.T) {
	repos, errs := Parse([]byte("---\n"))
	if len(repos) != 0 || len(errs) != 0 {
		t.Errorf("marker-only stream parsed to %d documents and %v, want none", len(repos), errs)
	}
}

// Error positions count every document in the stream, including empty ones,
// so "document N" matches what the user sees when counting separators.
func TestParseNumbersDocumentsByStreamPosition(t *testing.T) {
	bad := strings.ReplaceAll(validDoc, "kind: Repository", "kind: Nonsense")
	_, errs := Parse(joinDocs(validDoc, "", bad))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one error", errs)
	}
	if !strings.Contains(errs[0].Error(), "document 3") {
		t.Errorf("error %q should name document 3: the empty document keeps its position", errs[0])
	}
}

// Only a bare "---" line separates documents. A line that merely starts with
// dashes ("----", "--- inline") belongs to its document and must surface as
// that document's parse error, not shift every following document.
func TestParseTreatsDashLinesAsContent(t *testing.T) {
	repos, errs := Parse(joinDocs(validDoc, "kind: Broken\ndescription: x\n----\n", validDoc))
	if len(repos) != 2 {
		t.Errorf("parsed %d documents, want the 2 valid ones", len(repos))
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one error for the dash-line document", errs)
	}
	if !strings.Contains(errs[0].Error(), "document 2") {
		t.Errorf("error %q should blame document 2, not a phantom split at the dash line", errs[0])
	}
}

// YAML permits a comment after the document start marker ("--- # note"), so
// such a line must separate documents instead of leaving both documents in
// one fragment and surfacing a misleading inline-document error.
func TestParseSplitsOnSeparatorWithComment(t *testing.T) {
	second := strings.ReplaceAll(validDoc, "name: proj", "name: other")
	repos, errs := Parse([]byte(validDoc + "--- # environments\n" + second))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if len(repos) != 2 {
		t.Errorf("parsed %d documents, want 2", len(repos))
	}
}

// A "--- content" marker carries an in-line document that this parser does
// not accept: the dashes are not a bare separator, so splitStream keeps the
// line, and goyaml then splits the fragment into two documents.
func TestParseRejectsInlineDocumentMarker(t *testing.T) {
	_, errs := Parse([]byte(validDoc + "--- inline\n"))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one inline-document error", errs)
	}
	if !errors.Is(errs[0], errInlineDocument) {
		t.Errorf("error %q is not errInlineDocument", errs[0])
	}
}

// goyaml error positions must point at the file the user is editing, not at
// a document-relative fragment, so the reported line is found by searching
// the assembled stream for the offending key.
func TestParseReportsFileLineNumbers(t *testing.T) {
	stream := string(joinDocs(validDoc, validDoc+"  bogus_key: x\n"))
	wantLine := 0
	for index, line := range strings.Split(stream, "\n") {
		if strings.Contains(line, "bogus_key") {
			wantLine = index + 1
		}
	}

	_, errs := Parse([]byte(stream))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one unknown-key error", errs)
	}
	if want := fmt.Sprintf("[%d:", wantLine); !strings.Contains(errs[0].Error(), want) {
		t.Errorf("error %q does not point at file line %d", errs[0], wantLine)
	}
}
