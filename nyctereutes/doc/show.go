package doc

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
	docfs "github.com/Omochice/nyctereutes/doc"
)

var (
	errShowNeedsName  = errors.New("no document name given")
	errNoSuchDocument = errors.New("no such document")
)

// Backs the "doc show" subcommand.
type showCommand struct {
	inout *cli.ProcInout
}

func (c *showCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errShowNeedsName
	}
	fsys := docfs.Pages()
	page, err := fs.ReadFile(fsys, args[0]+ext)
	if err != nil {
		return notFound(fsys, args[0])
	}
	_, _ = fmt.Fprint(c.inout.Stdout, string(page))
	return nil
}

// Reports a name no page carries, listing the names that do exist.
func notFound(fsys fs.FS, name string) error {
	docs, err := documents(fsys)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", errNoSuchDocument, name, err)
	}
	names := make([]string, 0, len(docs))
	for _, doc := range docs {
		names = append(names, doc.Name)
	}
	return fmt.Errorf("%w: %s; available documents: %s", errNoSuchDocument, name, strings.Join(names, ", "))
}
