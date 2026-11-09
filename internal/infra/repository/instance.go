package repository

import (
	"context"
	"encoding/json"
	"fmt"
)

// The part of GET /version a read consults: which edition answers.
// A Community Edition build leaves whole fields out of its responses.
// What a response omits can only be told from what it never carries once the edition is known.
type instanceVersion struct {
	Enterprise bool `json:"enterprise"`
}

// Reports whether the instance is an Enterprise build.
// Asked of the instance rather than inferred from a project's resources.
// A project holding none of them says nothing about the edition.
func (c *Client) isEnterprise(ctx context.Context) (bool, error) {
	out, err := c.runner.Run(ctx, "api", "version")
	if err != nil {
		return false, fmt.Errorf("fetch instance version: %w", err)
	}
	var version instanceVersion
	if err := json.Unmarshal(out, &version); err != nil {
		return false, fmt.Errorf("parse instance version: %w", err)
	}
	return version.Enterprise, nil
}
