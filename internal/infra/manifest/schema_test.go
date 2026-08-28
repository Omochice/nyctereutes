package manifest

import (
	"bytes"
	"flag"
	"os"
	"testing"
)

// Rewrites the committed schema from the types instead of checking it, so the
// file and the check it must pass are produced by the same code.
//
//nolint:gochecknoglobals // a test flag has to be registered before testing parses flags
var updateSchema = flag.Bool("update", false, "rewrite schema/repository.schema.json from the manifest types")

// Where the generated schema is committed, relative to this package.
const schemaFile = "../../../schema/repository.schema.json"

func TestCommittedSchemaMatchesTypes(t *testing.T) {
	generated, err := Schema()
	if err != nil {
		t.Fatalf("Schema() error = %v", err)
	}
	if *updateSchema {
		if err := os.WriteFile(schemaFile, generated, 0o600); err != nil {
			t.Fatalf("write %s: %v", schemaFile, err)
		}
		return
	}
	committed, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("read %s: %v", schemaFile, err)
	}
	if !bytes.Equal(generated, committed) {
		t.Errorf("%s no longer matches the manifest types; regenerate it with"+
			" `go test ./internal/infra/manifest/ -update`", schemaFile)
	}
}
