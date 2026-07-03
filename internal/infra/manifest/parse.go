package manifest

import (
	"bytes"
	"errors"
	"fmt"

	goyaml "github.com/goccy/go-yaml"
)

var (
	// Signals a document whose kind this schema does not define.
	errUnknownKind = errors.New("unknown kind")
	// Signals a document written for a schema version this binary does not read.
	errUnsupportedAPIVersion = errors.New("unsupported apiVersion")
	// Signals a field the schema requires but the document leaves empty.
	errRequiredField = errors.New("required field is missing")
)

// Parses a YAML stream of "---"-separated manifest documents against the
// schema, continuing past a broken document so one mistake does not hide the
// rest. Returned errors carry the 1-based document position; the returned
// documents are the ones that validated.
func Parse(data []byte) ([]*Repository, []error) {
	var repos []*Repository
	var errs []error
	for index, doc := range splitDocuments(data) {
		repo, err := parseDocument(doc)
		if err != nil {
			errs = append(errs, fmt.Errorf("document %d: %w", index+1, err))
			continue
		}
		repos = append(repos, repo)
	}
	return repos, errs
}

// Splits a YAML stream on "---" separator lines, dropping blank documents.
// Spec values sit at least one indentation level deep, so a "---" line inside
// a literal block is always indented and never matches the separator.
func splitDocuments(data []byte) [][]byte {
	var docs [][]byte
	for _, doc := range bytes.Split(data, []byte("\n---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		docs = append(docs, doc)
	}
	return docs
}

// Validates one document. The kind and apiVersion are probed leniently first,
// so a foreign document is reported as such instead of as a pile of
// unknown-field errors; only then does the full schema decode strictly.
func parseDocument(data []byte) (*Repository, error) {
	var probe struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	if err := goyaml.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if probe.Kind != KindRepository {
		return nil, fmt.Errorf("%w %q", errUnknownKind, probe.Kind)
	}
	if probe.APIVersion != APIVersion {
		return nil, fmt.Errorf("%w %q", errUnsupportedAPIVersion, probe.APIVersion)
	}

	var repo Repository
	if err := goyaml.UnmarshalWithOptions(data, &repo, goyaml.DisallowUnknownField()); err != nil {
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
