package manifest

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// One fully spelled-out schedule, so a decode test can assert every attribute
// without a default standing in for a declared one.
const scheduleDoc = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  topics: []
  pipeline_schedules:
    - description: nightly
      ref: refs/heads/main
      cron: "0 3 * * *"
      cron_timezone: Asia/Tokyo
      active: false
`

// Varies only the ref, so a test about ref shapes states the shape it means and
// nothing else.
func scheduleDocWithRef(ref string) []byte {
	return []byte(`apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  topics: []
  pipeline_schedules:
    - description: nightly
      ref: ` + ref + `
      cron: "0 3 * * *"
`)
}

// GitLab reports a schedule's ref as a full path, so a manifest holding the
// short name a person types would otherwise read as permanent drift. Only the
// full form exists inside the program; a bare name is taken to mean a branch,
// which keeps a branch and a tag of the same name apart.
func TestParseCanonicalizesScheduleRef(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		declared string
		want     string
	}{
		{name: "bare name means a branch", declared: "main", want: "refs/heads/main"},
		{name: "branch path is kept", declared: "refs/heads/main", want: "refs/heads/main"},
		{name: "tag path is kept", declared: "refs/tags/v1.0.0", want: "refs/tags/v1.0.0"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repos, errs := Parse(scheduleDocWithRef(testCase.declared))
			if len(errs) > 0 {
				t.Fatalf("errs = %v, want none", errs)
			}
			if got := string(repos[0].Spec.PipelineSchedules[0].Ref); got != testCase.want {
				t.Errorf("ref = %q, want %q", got, testCase.want)
			}
		})
	}
}

// A declared schedule is total rather than partial: it can be created from the
// document alone, so an omitted attribute has to mean something. It means what
// GitLab would have used had the schedule been created through the API, which
// keeps one document from meaning two things depending on whether the schedule
// already exists.
func TestParseFillsOmittedScheduleAttributesWithGitLabDefaults(t *testing.T) {
	repos, errs := Parse(scheduleDocWithRef("main"))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}

	schedule := repos[0].Spec.PipelineSchedules[0]
	if schedule.CronTimezone != "UTC" {
		t.Errorf("cron_timezone = %q, want %q", schedule.CronTimezone, "UTC")
	}
	if !schedule.Active {
		t.Error("active = false, want true")
	}
}

// An explicitly declared value must survive the defaulting, or a schedule
// deliberately paused in the manifest would silently come back on.
func TestParseKeepsExplicitlyDeclaredScheduleAttributes(t *testing.T) {
	repos, errs := Parse([]byte(scheduleDoc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}

	schedule := repos[0].Spec.PipelineSchedules[0]
	if schedule.CronTimezone != "Asia/Tokyo" {
		t.Errorf("cron_timezone = %q, want %q", schedule.CronTimezone, "Asia/Tokyo")
	}
	if schedule.Active {
		t.Error("active = true, want false")
	}
}

// The two ways of writing "no schedules" mean different things, so decoding
// must keep them apart: an omitted block leaves the live schedules alone, while
// an empty list declares that none should exist.
func TestParseDistinguishesOmittedSchedulesFromAnEmptyList(t *testing.T) {
	omitted, errs := Parse([]byte(validDoc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if schedules := omitted[0].Spec.PipelineSchedules; schedules != nil {
		t.Errorf("omitted block = %v, want nil (unmanaged)", schedules)
	}

	declared, errs := Parse([]byte(validDoc + "  pipeline_schedules: []\n"))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	schedules := declared[0].Spec.PipelineSchedules
	if schedules == nil {
		t.Fatal("empty list = nil, want an empty slice (declares none)")
	}
	if len(schedules) != 0 {
		t.Errorf("empty list = %v, want no elements", schedules)
	}
}

// A schedule decodes itself, so the strict decoding the surrounding document
// gets does not reach inside it unless the nested decode asks for it too. A
// mistyped attribute would otherwise be dropped in silence.
func TestParseRejectsUnknownKeyInsideASchedule(t *testing.T) {
	doc := validDoc + `  pipeline_schedules:
    - description: nightly
      ref: main
      cron: "0 3 * * *"
      timezone: UTC
`
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "timezone") {
		t.Errorf("error = %q, want it to name the unknown key", errs[0])
	}
}

// A schedule omitting description, ref or cron is rejected at parse time, and
// the error names the omitted field and the schedule's position.
func TestParseRequiresScheduleFields(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		body  string
		field string
	}{
		{
			name:  "description omitted",
			body:  "      ref: main\n      cron: \"0 3 * * *\"\n",
			field: "description",
		},
		{
			name:  "description blank",
			body:  "      description: \"\"\n      ref: main\n      cron: \"0 3 * * *\"\n",
			field: "description",
		},
		{
			name:  "ref omitted",
			body:  "      description: nightly\n      cron: \"0 3 * * *\"\n",
			field: "ref",
		},
		{
			name:  "cron omitted",
			body:  "      description: nightly\n      ref: main\n",
			field: "cron",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, errs := Parse([]byte(validDoc + "  pipeline_schedules:\n    -\n" + testCase.body))
			if len(errs) != 1 {
				t.Fatalf("errs = %v, want one", errs)
			}
			if !strings.Contains(errs[0].Error(), testCase.field) {
				t.Errorf("error = %q, want it to name %q", errs[0], testCase.field)
			}
		})
	}
}

// The description is how a declared schedule is paired with a live one, and
// GitLab does not enforce uniqueness itself, so a document repeating one leaves
// the pairing undecidable. The error names the description because that is what
// the reader has to go and change.
func TestParseRejectsDuplicateScheduleDescriptions(t *testing.T) {
	doc := validDoc + `  pipeline_schedules:
    - description: nightly
      ref: main
      cron: "0 3 * * *"
    - description: weekly
      ref: main
      cron: "0 9 * * 1"
    - description: nightly
      ref: main
      cron: "0 8 * * *"
`
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "nightly") {
		t.Errorf("error = %q, want it to name the repeated description", errs[0])
	}
}

// An omitted block leaves the live schedules alone while an empty list declares
// that none should exist, so a document that means the latter has to say so.
// Dropping the key when the list is empty would emit the former instead, and
// Marshal's own round-trip check rejects such a document as lossy.
func TestMarshalKeepsAnEmptyScheduleList(t *testing.T) {
	doc := fullRepository()
	doc.Spec.PipelineSchedules = []PipelineSchedule{}

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "pipeline_schedules: []") {
		t.Errorf("emitted document does not declare an empty schedule list:\n%s", out)
	}
}

// An omitted active decodes as true, so a paused schedule survives an export
// only if the false is actually written. Were it dropped as an empty value, the
// export would silently turn the schedule back on.
func TestMarshalKeepsAPausedSchedule(t *testing.T) {
	doc := fullRepository()
	doc.Spec.PipelineSchedules = []PipelineSchedule{{
		Description:  "nightly",
		Ref:          "refs/heads/main",
		Cron:         "0 3 * * *",
		CronTimezone: defaultCronTimezone,
		Active:       false,
	}}

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	repos, errs := Parse(out)
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if repos[0].Spec.PipelineSchedules[0].Active {
		t.Errorf("active = true after a round trip, want false\n%s", out)
	}
}

// Every other decoding error points at the line the reader sees in their
// editor, so a mistyped schedule attribute has to as well. A schedule decoded
// from a detached byte slice reports a line counted from the schedule's own
// start, which sends the reader to an unrelated part of the file.
func TestParseReportsFileLineNumbersInsideASchedule(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		body    string
		offends string
	}{
		{
			name: "unknown attribute",
			body: `  pipeline_schedules:
    - description: nightly
      ref: main
      cron: "0 3 * * *"
      timezone: UTC
`,
			offends: "timezone:",
		},
		{
			name: "attribute of the wrong shape",
			body: `  pipeline_schedules:
    - description: nightly
      ref: [a, b]
      cron: "0 3 * * *"
`,
			offends: "ref:",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stream := string(joinDocs(validDoc, validDoc+testCase.body))
			wantLine := 0
			for index, line := range strings.Split(stream, "\n") {
				if strings.Contains(line, testCase.offends) {
					wantLine = index + 1
				}
			}

			_, errs := Parse([]byte(stream))
			if len(errs) != 1 {
				t.Fatalf("errs = %v, want exactly one error", errs)
			}
			if want := fmt.Sprintf("[%d:", wantLine); !strings.Contains(errs[0].Error(), want) {
				t.Errorf("error %q does not point at file line %d", errs[0], wantLine)
			}
		})
	}
}

func TestParseReadsPipelineSchedules(t *testing.T) {
	repos, errs := Parse([]byte(scheduleDoc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if len(repos) != 1 {
		t.Fatalf("parsed %d documents, want 1", len(repos))
	}

	schedules := repos[0].Spec.PipelineSchedules
	if len(schedules) != 1 {
		t.Fatalf("schedules = %d, want 1", len(schedules))
	}
	got := schedules[0]
	want := PipelineSchedule{
		Description:  "nightly",
		Ref:          "refs/heads/main",
		Cron:         "0 3 * * *",
		CronTimezone: "Asia/Tokyo",
		Active:       false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("schedule = %+v, want %+v", got, want)
	}
}

// A document whose schedule holds a bare ref still marshals, and the emitted
// ref is the expanded form.
func TestMarshalSurvivesABareRef(t *testing.T) {
	doc := fullRepository()
	doc.Spec.PipelineSchedules[0].Ref = "main"

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "ref: refs/heads/main") {
		t.Errorf("emitted document does not carry the canonical ref:\n%s", out)
	}
}
