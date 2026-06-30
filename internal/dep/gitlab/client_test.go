package gitlab

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
)

// detailPath matches a single merge request detail endpoint (no query string).
var detailPath = regexp.MustCompile(`merge_requests/\d+$`)

func TestSearchMRsConvertsResultsAndFetchesStatus(t *testing.T) {
	list := `[{"iid":12,"project_id":7,"title":"Bump lodash from 1.0.0 to 2.0.0",` +
		`"author":{"username":"renovate-bot"},` +
		`"web_url":"https://gitlab.com/g/proj/-/merge_requests/12","sha":"abc"}]`
	detail := `{"head_pipeline":{"status":"success"},"has_conflicts":false,"detailed_merge_status":"mergeable"}`

	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		path := args[len(args)-1]
		if detailPath.MatchString(path) {
			return []byte(detail), nil
		}
		return []byte(list), nil
	})

	mrs, err := NewClient(runner).SearchMRs(context.Background(), SearchParams{
		Authors: []string{DefaultAuthor},
	})
	if err != nil {
		t.Fatalf("SearchMRs() error = %v", err)
	}

	if len(mrs) != 1 {
		t.Fatalf("got %d MRs, want 1", len(mrs))
	}
	got := mrs[0]
	if got.IID != 12 || got.ProjectID != 7 {
		t.Errorf("IID/ProjectID = %d/%d, want 12/7", got.IID, got.ProjectID)
	}
	if got.Project != "g/proj" {
		t.Errorf("Project = %q, want %q", got.Project, "g/proj")
	}
	if got.CIStatus != "success" {
		t.Errorf("CIStatus = %q, want %q", got.CIStatus, "success")
	}
	if got.UnmergeableReason != "" {
		t.Errorf("UnmergeableReason = %q, want empty", got.UnmergeableReason)
	}
}

func TestSearchMRsDeduplicatesAcrossAuthors(t *testing.T) {
	// Both authors return the same MR; the result must contain it once.
	list := `[{"iid":12,"project_id":7,"title":"Bump x from 1 to 2.0.0",` +
		`"web_url":"https://gitlab.com/g/proj/-/merge_requests/12"}]`
	detail := `{"head_pipeline":{"status":"running"}}`

	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		path := args[len(args)-1]
		if detailPath.MatchString(path) {
			return []byte(detail), nil
		}
		return []byte(list), nil
	})

	mrs, err := NewClient(runner).SearchMRs(context.Background(), SearchParams{
		Authors: []string{"bot-a", "bot-b"},
	})
	if err != nil {
		t.Fatalf("SearchMRs() error = %v", err)
	}
	if len(mrs) != 1 {
		t.Fatalf("got %d MRs, want 1 (deduplicated)", len(mrs))
	}
	if mrs[0].CIStatus != "pending" {
		t.Errorf("CIStatus = %q, want %q (running normalizes to pending)", mrs[0].CIStatus, "pending")
	}
}

func TestSearchMRsRequestsConfiguredAuthor(t *testing.T) {
	var sawAuthor string
	runner := glab.RunnerFunc(func(_ context.Context, args ...string) ([]byte, error) {
		path := args[len(args)-1]
		if detailPath.MatchString(path) {
			return []byte(`{}`), nil
		}
		if _, after, found := strings.Cut(path, "author_username="); found {
			sawAuthor, _, _ = strings.Cut(after, "&")
		}
		return []byte(`[]`), nil
	})

	_, err := NewClient(runner).SearchMRs(context.Background(), SearchParams{
		Authors: []string{DefaultAuthor},
	})
	if err != nil {
		t.Fatalf("SearchMRs() error = %v", err)
	}
	if sawAuthor != DefaultAuthor {
		t.Errorf("author_username query = %q, want %q", sawAuthor, DefaultAuthor)
	}
}
