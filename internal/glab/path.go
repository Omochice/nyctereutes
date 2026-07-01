package glab

import "net/url"

// Percent-encodes a GitLab path for embedding as a single segment in a
// `glab api` endpoint. The GitLab REST API addresses both projects
// (`projects/:id`) and files (`repository/files/:file_path`) by a
// namespace-qualified path in which the "/" separators must be escaped to %2F so
// the whole value is treated as one path component. [url.PathEscape] already
// escapes "/" (it is reserved inside a path segment).
func EncodePath(s string) string {
	return url.PathEscape(s)
}
