package gitlab

import "strings"

// DefaultAuthor is the GitLab username searched for when neither the --author
// flag nor dep.author is set. Renovate is the de facto dependency bot on GitLab.
const DefaultAuthor = "renovate-bot"

// ResolveAuthors picks the effective author filter. A non-nil author flag wins
// (even when empty, which is an explicit "any author"), then dep.author, then
// the default Renovate bot username. A nil flag means "not specified".
func ResolveAuthors(author *string, cfgAuthor string) []string {
	if author != nil {
		return []string{*author}
	}
	if cfgAuthor != "" {
		return []string{cfgAuthor}
	}
	return []string{DefaultAuthor}
}

// ResolveScope determines the group and project targets. An explicit --repo
// flag wins over dep.repo; groupPath is passed through as the group filter.
// When both repo sources are empty, callers search across all accessible
// projects.
func ResolveScope(repo *string, groupPath *string, cfgRepos []string) (group string, repos []string) {
	if repo != nil {
		repos = cleanList(*repo)
	} else {
		repos = cfgRepos
	}
	if groupPath != nil {
		group = *groupPath
	}
	return group, repos
}

// cleanList splits a comma-separated value, trimming blanks.
func cleanList(value string) []string {
	var items []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			items = append(items, part)
		}
	}
	return items
}
