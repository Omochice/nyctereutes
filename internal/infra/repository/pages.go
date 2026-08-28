package repository

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Joins the pages glab wrote. In --paginate mode it emits one JSON array per
// page back to back rather than a single merged array, so the whole output is
// not one JSON value and has to be decoded in sequence.
//
// Output holding no page, or a page written as null, is refused with empty
// rather than read as a project owning none of the resource: the child-resource
// fields on [manifest.RepositorySpec] would carry that into a document as a
// declaration that none may exist.
func decodePages[Item any](out []byte, empty error) ([]Item, error) {
	decoder := json.NewDecoder(bytes.NewReader(out))
	items := []Item{}
	pages := 0
	for {
		var page []Item
		if err := decoder.Decode(&page); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode page: %w", err)
		}
		if page == nil {
			return nil, empty
		}
		items = append(items, page...)
		pages++
	}
	if pages == 0 {
		return nil, empty
	}
	return items, nil
}
