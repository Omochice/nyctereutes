package manifest

import (
	"bytes"
	"errors"
	"fmt"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

var (
	// Signals a document whose kind this schema does not define.
	errUnknownKind = errors.New("unknown kind")
	// Signals a document written for a schema version this binary does not read.
	errUnsupportedAPIVersion = errors.New("unsupported apiVersion")
	// Signals a field the schema requires but the document leaves empty.
	errRequiredField = errors.New("required field is missing")
	// Signals an in-line "--- content" marker, which this parser does not accept.
	errInlineDocument = errors.New("unsupported inline document after ---")
	// Reports a document holding nothing but comments or whitespace; the
	// caller skips it the way it skips a blank one.
	errBlankDocument = errors.New("blank document")
)

// Parses a YAML stream of "---"-separated manifest documents against the
// schema, continuing past a broken document so one mistake does not hide the
// rest. Returned errors carry the 1-based document position; the returned
// documents are the ones that validated.
//
// The stream is split here rather than handed to goyaml whole: goyaml
// v1.19.2 silently drops every document after a consecutive-separator empty
// document (in both the parser and decoder APIs), and a validator must never
// lose sight of a document. Each fragment is then parsed on its own, so a
// syntax error is also confined to its document.
func Parse(data []byte) ([]*Repository, []error) {
	// A leading UTF-8 BOM (written by some Windows editors, permitted at
	// stream start by the YAML spec) would otherwise glue itself onto the
	// first key and surface as a baffling unsupported-apiVersion error.
	data = bytes.TrimPrefix(data, []byte("\ufeff"))

	var repos []*Repository
	var errs []error
	for _, frag := range splitStream(data) {
		repo, err := parseDocument(frag)
		if errors.Is(err, errBlankDocument) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("document %d: %w", frag.number, err))
			continue
		}
		repos = append(repos, repo)
	}
	return repos, errs
}

// One "---"-delimited chunk of a stream: its content, its 1-based document
// position, and the file line its content starts on.
type documentFragment struct {
	body      []byte
	number    int
	startLine int
}

// Splits a stream on bare "---" separator lines. Only a line that is exactly
// the marker separates documents; a line that merely starts with dashes
// ("----", "--- inline") belongs to its document and is judged by the YAML
// parser. The chunk before the first separator is special: when blank, the
// marker is the first document's own opening and no document exists there,
// while a later blank chunk is a real (empty) document that keeps its
// position in the numbering.
func splitStream(data []byte) []documentFragment {
	var fragments []documentFragment
	var current [][]byte
	currentStart := 1
	number := 0
	leading := true

	flush := func(nextStart int) {
		body := bytes.Join(current, []byte("\n"))
		blank := len(bytes.TrimSpace(body)) == 0
		if !leading || !blank {
			number++
			if !blank {
				fragments = append(fragments, documentFragment{body: body, number: number, startLine: currentStart})
			}
		}
		leading = false
		current = nil
		currentStart = nextStart
	}

	for index, line := range bytes.Split(data, []byte("\n")) {
		if isSeparator(line) {
			separatorLine := index + 1
			flush(separatorLine + 1)
			continue
		}
		current = append(current, line)
	}
	flush(0)
	return fragments
}

// Reports whether line is a bare document separator.
func isSeparator(line []byte) bool {
	return bytes.Equal(bytes.TrimRight(line, " \t\r"), []byte("---"))
}

// Validates one document. The fragment is padded with newlines up to its
// position in the file before parsing, so every goyaml error annotation
// points at the line the user sees in their editor, not at a
// fragment-relative one. The kind and apiVersion are probed leniently first,
// so a foreign document is reported as such instead of as a pile of
// unknown-field errors; only then does the full schema decode strictly.
func parseDocument(frag documentFragment) (*Repository, error) {
	padded := make([]byte, 0, frag.startLine-1+len(frag.body))
	padded = append(padded, bytes.Repeat([]byte("\n"), frag.startLine-1)...)
	padded = append(padded, frag.body...)

	file, err := parser.ParseBytes(padded, 0)
	if err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if len(file.Docs) > 1 {
		return nil, errInlineDocument
	}
	if len(file.Docs) == 0 || file.Docs[0].Body == nil {
		return nil, errBlankDocument
	}
	body := file.Docs[0].Body

	var probe struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	if err := goyaml.NodeToValue(body, &probe); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if probe.Kind != KindRepository {
		return nil, fmt.Errorf("%w %q", errUnknownKind, probe.Kind)
	}
	if probe.APIVersion != APIVersion {
		return nil, fmt.Errorf("%w %q", errUnsupportedAPIVersion, probe.APIVersion)
	}

	var repo Repository
	if err := goyaml.NodeToValue(body, &repo, goyaml.DisallowUnknownField()); err != nil {
		return nil, fmt.Errorf("parse %s: %w", KindRepository, err)
	}
	if err := repo.validate(); err != nil {
		return nil, err
	}
	return &repo, nil
}

// The required-field checks the schema types cannot express themselves:
// metadata must address a GitLab project.
func (repo *Repository) validate() error {
	if repo.Metadata.Name == "" {
		return fmt.Errorf("%w: metadata.name", errRequiredField)
	}
	if repo.Metadata.Owner == "" {
		return fmt.Errorf("%w: metadata.owner", errRequiredField)
	}
	return nil
}
