package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/types"
)

func TestDisplayListRendersTable(t *testing.T) {
	var buf bytes.Buffer
	mrs := []types.MR{
		{IID: 12, Project: "g/proj", Title: "Bump lodash from 1.0.0 to 2.0.0"},
	}
	if err := New(&buf, mrs, false).DisplayList(mrs); err != nil {
		t.Fatalf("DisplayList() error = %v", err)
	}
	out := buf.String()
	for _, want := range []string{"PROJECT", "MR", "TITLE", "g/proj", "!12", "Bump lodash"} {
		if !strings.Contains(out, want) {
			t.Errorf("DisplayList output missing %q\n%s", want, out)
		}
	}
}

func TestDisplayListJSON(t *testing.T) {
	var buf bytes.Buffer
	mrs := []types.MR{{IID: 12, Project: "g/proj", Title: "t"}}
	if err := New(&buf, mrs, true).DisplayList(mrs); err != nil {
		t.Fatalf("DisplayList() error = %v", err)
	}
	var decoded []types.MR
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(decoded) != 1 || decoded[0].IID != 12 {
		t.Errorf("decoded = %+v, want one MR with IID 12", decoded)
	}
}

func TestDisplayGroupsSortedAlphabetically(t *testing.T) {
	var buf bytes.Buffer
	groups := map[string][]types.MR{
		"zlib@1.0.0":  {{IID: 1, Project: "g/z", URL: "u1"}},
		"alpha@2.0.0": {{IID: 2, Project: "g/a", URL: "u2"}},
	}
	if err := NewFromGroups(&buf, groups, false).DisplayGroups(groups); err != nil {
		t.Fatalf("DisplayGroups() error = %v", err)
	}
	out := buf.String()
	if strings.Index(out, "alpha@2.0.0") > strings.Index(out, "zlib@1.0.0") {
		t.Errorf("groups not sorted alphabetically:\n%s", out)
	}
}

func TestPrintActionMultiProjectPrefix(t *testing.T) {
	var buf bytes.Buffer
	mrs := []types.MR{
		{IID: 12, Project: "g/a"},
		{IID: 13, Project: "g/b"},
	}
	u := New(&buf, mrs, false)
	u.PrintAction("approve", mrs[0])
	out := buf.String()
	if !strings.Contains(out, "[g/a] approve !12") {
		t.Errorf("want multi-project prefix, got %q", out)
	}
}

func TestPrintActionSingleProjectNoPrefix(t *testing.T) {
	var buf bytes.Buffer
	mrs := []types.MR{{IID: 12, Project: "g/a"}}
	u := New(&buf, mrs, false)
	u.PrintAction("approve", mrs[0])
	out := buf.String()
	if strings.Contains(out, "[g/a]") {
		t.Errorf("single project should have no prefix, got %q", out)
	}
	if !strings.Contains(out, "approve !12") {
		t.Errorf("want action message, got %q", out)
	}
}
