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
	// This reflector looks the JSONSchema and JSONSchemaExtend hooks up on the
	// value type, which is why every such method in this package takes a value
	// receiver while the decoding methods beside them take a pointer.
	reflector := jsonschema.Reflector{
		// The types carry no json tags at all, so the default would name every
		// document key after its Go identifier.
		FieldNameTag: "yaml",
		// Several optional fields omit omitempty so the emitter always writes
		// them; inferring required from omitempty would turn those into demands.
		RequiredFromJSONSchemaTags: true,
		// The generated $id would be this package's Go import path, which is
		// neither the schema's published location nor resolvable.
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
