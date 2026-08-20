package manifest

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// Derives the JSON Schema of a [Repository] document from the types above, as
// indented JSON ending in a newline. Deriving it rather than maintaining a
// hand-written schema is what keeps the rules an editor applies and the rules
// the parser applies from drifting apart.
func Schema() ([]byte, error) {
	reflector := jsonschema.Reflector{
		// The document keys come from the yaml tags; the types carry no json
		// tags at all, so the default would name every field after its Go
		// identifier.
		FieldNameTag: "yaml",
		// Required is stated explicitly rather than inferred from omitempty:
		// several optional fields deliberately omit it so the emitter always
		// writes them, and inference would turn those into demands.
		RequiredFromJSONSchemaTags: true,
		// The generated $id would be the Go import path of this package, an
		// internal detail that is neither the schema's published location nor
		// resolvable.
		Anonymous: true,
	}
	schema, err := json.MarshalIndent(reflector.Reflect(&Repository{}), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json schema: %w", err)
	}
	return append(schema, '\n'), nil
}

// Where the derived schema is published, split around the git ref so a document
// can name the revision it was written by.
const (
	schemaURLPrefix = "https://raw.githubusercontent.com/Omochice/nyctereutes/"
	schemaURLPath   = "/schema/repository.schema.json"
)

// The yaml-language-server modeline naming the schema committed at the given
// git ref, as a whole comment line terminated by a newline so a caller can
// write it straight ahead of a document. Pinning the ref rather than a moving
// branch keeps the rules an editor applies to an emitted document those of the
// revision that emitted it.
//
// An editor only reads the modeline out of a single document's leading comment
// block, so a caller emitting a "---"-separated stream repeats the line for
// every document instead of writing it once.
func SchemaModeline(ref string) string {
	return "# yaml-language-server: $schema=" + schemaURLPrefix + ref + schemaURLPath + "\n"
}
