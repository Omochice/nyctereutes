package gitlab

import (
	"testing"

	"github.com/Omochice/nyctereutes/internal/types"
)

const projectA = "g/a"

func TestGroupMRsBucketsByPackageVersion(t *testing.T) {
	mrs := []types.MR{
		{IID: 1, Project: projectA, Title: "Bump lodash from 4.0.0 to 4.17.21"},
		{IID: 2, Project: "g/b", Title: "Bump lodash from 4.1.0 to 4.17.21"},
		{IID: 3, Project: projectA, Title: "Update dependency typescript to 5.6.0"},
	}

	groups := GroupMRs(mrs, nil)

	if got := len(groups["lodash@4.17.21"]); got != 2 {
		t.Errorf("lodash@4.17.21 group size = %d, want 2", got)
	}
	if got := len(groups["typescript@5.6.0"]); got != 1 {
		t.Errorf("typescript@5.6.0 group size = %d, want 1", got)
	}
}

func TestGroupMRsRejectsEmptyCaptureKey(t *testing.T) {
	// The custom pattern matches the title but captures an empty package.
	// Before the fix this produced a malformed "@1.2.3" group key; now the MR
	// falls back to a unique unparsed key.
	mrs := []types.MR{{IID: 1, Project: projectA, Title: "[] 1.2.3"}}

	groups := GroupMRs(mrs, []string{`^\[(\w*)\]\s+(v?\d\S*)`})

	if _, bad := groups["@1.2.3"]; bad {
		t.Errorf("empty package capture produced a malformed key @1.2.3: %v", groups)
	}
	if _, ok := groups["unparsed:g/a!1"]; !ok {
		t.Errorf("want the MR under a unique unparsed key, got %v", groups)
	}
}

func TestGroupKeyFuncMatchesGroupMRs(t *testing.T) {
	groupKey := GroupKeyFunc(nil)

	parsed := types.MR{IID: 1, Project: projectA, Title: "Bump lodash from 4.0.0 to 4.17.21"}
	if got := groupKey(parsed); got != "lodash@4.17.21" {
		t.Errorf("group key = %q, want lodash@4.17.21", got)
	}

	unparsed := types.MR{IID: 7, Project: "g/c", Title: "Refactor the build"}
	if got := groupKey(unparsed); got != "unparsed:g/c!7" {
		t.Errorf("group key for an unparsed title = %q, want unparsed:g/c!7", got)
	}
}

func TestGroupMRsKeepsUnparsedMRsSeparate(t *testing.T) {
	mrs := []types.MR{
		{IID: 1, Project: projectA, Title: "Refactor the build"},
		{IID: 2, Project: "g/b", Title: "Chore: cleanup"},
	}

	groups := GroupMRs(mrs, nil)

	if len(groups) != 2 {
		t.Fatalf("unparsed MRs should not share a group, got %d groups: %v", len(groups), groups)
	}
	for key, group := range groups {
		if len(group) != 1 {
			t.Errorf("group %q has %d MRs, want 1", key, len(group))
		}
	}
}
