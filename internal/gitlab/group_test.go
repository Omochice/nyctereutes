package gitlab

import (
	"testing"

	"github.com/Omochice/nyctereutes/internal/types"
)

func TestGroupMRsBucketsByPackageVersion(t *testing.T) {
	mrs := []types.MR{
		{IID: 1, Project: "g/a", Title: "Bump lodash from 4.0.0 to 4.17.21"},
		{IID: 2, Project: "g/b", Title: "Bump lodash from 4.1.0 to 4.17.21"},
		{IID: 3, Project: "g/a", Title: "Update dependency typescript to 5.6.0"},
	}

	groups := GroupMRs(mrs, nil)

	if got := len(groups["lodash@4.17.21"]); got != 2 {
		t.Errorf("lodash@4.17.21 group size = %d, want 2", got)
	}
	if got := len(groups["typescript@5.6.0"]); got != 1 {
		t.Errorf("typescript@5.6.0 group size = %d, want 1", got)
	}
}
