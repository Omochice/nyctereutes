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
	// The reflector finds the JSONSchema and JSONSchemaExtend hooks on the value
	// type only, which is why they take value receivers beside pointer-receiver
	// decoding methods.
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

const schemaURLFormat = "https://raw.githubusercontent.com/Omochice/nyctereutes/%s/schema/repository.schema.json"

// The yaml-language-server modeline naming the schema committed at the given
// git ref, terminated by a newline so a caller can write it straight ahead of a
// document. An editor reads the modeline out of a single document's leading
// comment block, so a caller emitting a "---"-separated stream repeats the line
// for every document instead of writing it once.
func SchemaModeline(ref string) string {
	return fmt.Sprintf("# yaml-language-server: $schema="+schemaURLFormat+"\n", ref)
}
