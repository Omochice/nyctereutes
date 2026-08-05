package doc

import (
	"encoding/json"
	"fmt"

	"github.com/Omochice/nyctereutes/cli"
	docfs "github.com/Omochice/nyctereutes/doc"
)

// Backs the "doc list" subcommand.
type listCommand struct {
	inout *cli.ProcInout
}

// What `doc list` writes. The help string travels with the results so a reader
// that has only this output still learns how to read a page.
type listing struct {
	Results []document `json:"results"`
	Help    string     `json:"help"`
}

const listHelp = "Run `nyctereutes doc show <name>` to read one of these documents."

func (c *listCommand) Execute(_ []string) error {
	docs, err := documents(docfs.FS)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(c.inout.Stdout)
	encoder.SetIndent("", "  ")
	// The help string is meant to be copied into a terminal, and the default
	// escaping would render its placeholder as an escape sequence.
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(listing{Results: docs, Help: listHelp}); err != nil {
		return fmt.Errorf("encode the documentation as JSON: %w", err)
	}
	return nil
}
