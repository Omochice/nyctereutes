package nyctereutes

import (
	"fmt"
	"io"

	"github.com/Omochice/nyctereutes/internal/infra/repository"
)

// printChanges writes one project's header followed by its indented change
// lines.
func printChanges(w io.Writer, name string, changes []repository.Change) {
	_, _ = fmt.Fprintf(w, "%s\n", name)
	for _, change := range changes {
		_, _ = fmt.Fprintf(w, "  %s\n", change)
	}
}
