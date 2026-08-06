package doc

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/Omochice/nyctereutes/cli"
)

// Backs the "doc list" subcommand.
type listCommand struct {
	inout *cli.ProcInout
	pages fs.FS
}

// What `doc list` writes.
type listing struct {
	Results []document `json:"results"`
	Help    string     `json:"help"`
}

const listHelp = "Run `nyctereutes doc show <name>` to read one of these documents."

func (c *listCommand) Execute(_ []string) error {
	docs, problems, err := documents(c.pages)
	if err != nil {
		return err
	}
	for _, problem := range problems {
		_, _ = fmt.Fprintln(c.inout.Stderr, problem)
	}
	encoder := json.NewEncoder(c.inout.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(listing{Results: docs, Help: listHelp}); err != nil {
		return fmt.Errorf("encode the documentation as JSON: %w", err)
	}
	return nil
}
