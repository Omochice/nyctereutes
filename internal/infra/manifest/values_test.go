package manifest

import (
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
)

// Values inside each enum's allowed set decode into the typed field
// unchanged, so validation does not get in the way of legitimate manifests.
func TestEnumValuesAcceptAllowedValues(t *testing.T) {
	cases := []struct{ name, doc string }{
		{"access_level_disabled", "features:\n  issues: disabled"},
		{"access_level_private", "features:\n  issues: private"},
		{"access_level_enabled", "features:\n  issues: enabled"},
		{"pages_accepts_public", "features:\n  pages: public"},
		{"package_registry_accepts_public", "features:\n  package_registry: public"},
		{"visibility_private", "visibility: private"},
		{"visibility_internal", "visibility: internal"},
		{"visibility_public", "visibility: public"},
	}
	for _, attr := range cases {
		t.Run(attr.name, func(t *testing.T) {
			var spec RepositorySpec
			if err := goyaml.Unmarshal([]byte(attr.doc), &spec); err != nil {
				t.Errorf("%q rejected: %v", attr.doc, err)
			}
		})
	}
}

// A value outside the allowed set fails to decode with an error listing what
// is allowed. The three-value access level must reject "public" even though
// the pages/package_registry type accepts it, so `issues: public` cannot
// slip through.
func TestEnumValuesRejectUnknownValues(t *testing.T) {
	cases := []struct{ name, doc, wantAllowed string }{
		{"access_level_rejects_public", "features:\n  issues: public", "disabled, private, enabled"},
		{"public_access_level_rejects_unknown", "features:\n  pages: open", "disabled, private, enabled, public"},
		{"visibility_rejects_unknown", "visibility: hidden", "private, internal, public"},
	}
	for _, attr := range cases {
		t.Run(attr.name, func(t *testing.T) {
			var spec RepositorySpec
			err := goyaml.Unmarshal([]byte(attr.doc), &spec)
			if err == nil {
				t.Fatalf("%q decoded, want a validation error", attr.doc)
			}
			if !strings.Contains(err.Error(), attr.wantAllowed) {
				t.Errorf("error %q does not list the allowed values %q", err, attr.wantAllowed)
			}
		})
	}
}

// Marshal re-decodes its output to verify the round trip, so an enum value
// outside the schema fails the import loudly instead of emitting a document
// that validate would then reject. The failure must name the offending value:
// a bare "does not survive a yaml round trip" gives the user nothing to fix.
func TestMarshalRejectsUnknownAccessLevel(t *testing.T) {
	doc := fullRepository()
	doc.Spec.Features.Issues = new(AccessLevel("beta"))

	_, err := Marshal(doc)
	if err == nil {
		t.Fatal("marshal succeeded for an unknown access level, want a round-trip failure")
	}
	if !strings.Contains(err.Error(), `"beta"`) {
		t.Errorf("error %q does not name the offending value", err)
	}
}
