package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

var errTopicCasing = errors.New("topic casing differs from the one GitLab holds")

// Rejects a manifest declaring a topic GitLab holds under another casing.
// GitLab resolves a name case-insensitively to the record its organization
// already holds, so the manifest is the side that can change.
func (c *Client) CheckTopicCasing(
	ctx context.Context, desired *manifest.Repository, current *CurrentState,
) error {
	if current.IsNew {
		return nil
	}
	carried := make(map[string]string, len(current.Topics))
	for _, name := range current.Topics {
		carried[strings.ToLower(name)] = name
	}

	var mismatches []string
	var readErr error
	for _, declared := range desired.Spec.Topics {
		held, known := carried[strings.ToLower(declared)]
		if !known && readErr == nil {
			held, readErr = c.fetchHeldTopic(ctx, declared)
		}
		if held != "" && held != declared {
			mismatches = append(mismatches, fmt.Sprintf("declared %q, held as %q", declared, held))
		}
	}

	var casingErr error
	if len(mismatches) > 0 {
		casingErr = fmt.Errorf("%w: %s", errTopicCasing, strings.Join(mismatches, "; "))
	}
	if err := errors.Join(casingErr, readErr); err != nil {
		return fmt.Errorf("check %s %s/%s: %w",
			fieldTopics, desired.Metadata.Owner, desired.Metadata.Name, err)
	}
	return nil
}

var errNoTopicPage = errors.New("response carries no topic list")

// The search lists the closest name first, so the topic named alike is
// expected on the first page.
// Following the pages would read every topic an instance holds under a common
// term.
const topicSearchPageSize = "100"

// Reads the spelling GitLab holds for name, which is empty when it holds no
// such topic.
func (c *Client) fetchHeldTopic(ctx context.Context, name string) (string, error) {
	out, err := c.runner.Run(ctx, "api",
		"topics?per_page="+topicSearchPageSize+"&search="+url.QueryEscape(name))
	if err != nil {
		return "", fmt.Errorf("fetch topic %q: %w", name, err)
	}
	var page []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &page); err != nil {
		return "", fmt.Errorf("parse topic %q: %w", name, err)
	}
	if page == nil {
		return "", fmt.Errorf("read topic %q: %w", name, errNoTopicPage)
	}
	for _, topic := range page {
		if strings.EqualFold(topic.Name, name) {
			return topic.Name, nil
		}
	}
	return "", nil
}
