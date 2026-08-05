package doc

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/Omochice/nyctereutes/cli"
)

var (
	errShowNeedsName  = errors.New("no document name given")
	errShowOneName    = errors.New("doc show reads one document at a time")
	errNoSuchDocument = errors.New("no such document")
)

// Backs the "doc show" subcommand.
type showCommand struct {
	inout *cli.ProcInout
	pages fs.FS
}

func (c *showCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errShowNeedsName
	}
	if len(args) > 1 {
		return fmt.Errorf("%w: %s", errShowOneName, strings.Join(args[1:], ", "))
	}
	page, err := fs.ReadFile(c.pages, args[0]+ext)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return notFound(c.pages, args[0])
		}
		return fmt.Errorf("read the document %s: %w", args[0], err)
	}
	_, _ = fmt.Fprint(c.inout.Stdout, prose(page))
	return nil
}

// Reports a name no page carries, listing the names that do exist. The names
// come from the filesystem rather than from the described pages, because show
// prints a page whether or not its frontmatter can be read.
func notFound(fsys fs.FS, name string) error {
	available, err := names(fsys)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", errNoSuchDocument, name, err)
	}
	return fmt.Errorf("%w: %s; available documents: %s", errNoSuchDocument, name, strings.Join(available, ", "))
}
