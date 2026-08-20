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
