package glab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Splits the output of "glab api --paginate" into its pages. glab emits one
// JSON array per page back to back rather than a single merged array, so the
// whole output is not one JSON value and has to be decoded in sequence. A page
// written as null is returned as a nil slice so a caller can tell it from an
// empty one.
func DecodePages[T any](out []byte) ([][]T, error) {
	decoder := json.NewDecoder(bytes.NewReader(out))
	var pages [][]T
	for {
		var page []T
		if err := decoder.Decode(&page); err != nil {
			if errors.Is(err, io.EOF) {
				return pages, nil
			}
			return nil, fmt.Errorf("decode page: %w", err)
		}
		pages = append(pages, page)
	}
}
