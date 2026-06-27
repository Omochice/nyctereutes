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
		Authors: []string{"renovate-bot"},
	})
	if err != nil {
		t.Fatalf("SearchMRs() error = %v", err)
	}

	if len(mrs) != 1 {
		t.Fatalf("got %d MRs, want 1", len(mrs))
	}
	mr := mrs[0]
	if mr.IID != 12 || mr.ProjectID != 7 {
		t.Errorf("IID/ProjectID = %d/%d, want 12/7", mr.IID, mr.ProjectID)
	}
	if mr.Project != "g/proj" {
		t.Errorf("Project = %q, want %q", mr.Project, "g/proj")
	}
	if mr.CIStatus != "success" {
		t.Errorf("CIStatus = %q, want %q", mr.CIStatus, "success")
	}
	if mr.UnmergeableReason != "" {
		t.Errorf("UnmergeableReason = %q, want empty", mr.UnmergeableReason)
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
		if i := strings.Index(path, "author_username="); i >= 0 {
			rest := path[i+len("author_username="):]
			if amp := strings.IndexByte(rest, '&'); amp >= 0 {
				rest = rest[:amp]
			}
			sawAuthor = rest
		}
		return []byte(`[]`), nil
	})

	_, err := NewClient(runner).SearchMRs(context.Background(), SearchParams{
		Authors: []string{"renovate-bot"},
	})
	if err != nil {
		t.Fatalf("SearchMRs() error = %v", err)
	}
	if sawAuthor != "renovate-bot" {
		t.Errorf("author_username query = %q, want %q", sawAuthor, "renovate-bot")
	}
}
