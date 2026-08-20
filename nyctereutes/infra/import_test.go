package infra_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/cli"
	"github.com/Omochice/nyctereutes/nyctereutes/infra"
)

func TestInfraImportEmitsYAML(t *testing.T) {
	runner := importFake(map[string]string{targetGroupProj: projJSON})
	exit, stdout, _ := runWithRunner(runner, "infra", "import", targetGroupProj)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	for _, want := range []string{"kind: Repository", "name: proj", "owner: group", "a tool"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

func TestInfraImportEmitsFeatureAccessLevels(t *testing.T) {
	withFeatures := `{"description":"d","visibility":"private","topics":[],"archived":false,` +
		`"issues_access_level":"enabled","wiki_access_level":"disabled","snippets_access_level":"private",` +
		`"builds_access_level":"enabled","merge_requests_access_level":"private","container_registry_access_level":"enabled"}`
	runner := importFake(map[string]string{targetGroupProj: withFeatures})
	exit, stdout, _ := runWithRunner(runner, "infra", "import", targetGroupProj)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	// Multi-word feature keys are snake_case, matching gh-infra and the GitLab API;
	// ci maps from builds_access_level.
	for _, want := range []string{
		"features:",
		"issues: enabled",
		"wiki: disabled",
		"snippets: private",
		"ci: enabled",
		"merge_requests: private",
		"container_registry: enabled",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
}

func TestInfraImportOmitsEmptyFeatures(t *testing.T) {
	noFeatures := `{"description":"d","visibility":"private","topics":[],"archived":false}`
	runner := importFake(map[string]string{targetGroupProj: noFeatures})
	exit, stdout, _ := runWithRunner(runner, "infra", "import", targetGroupProj)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if strings.Contains(stdout, "features:") {
		t.Errorf("a project with no access levels should omit the features block, not emit 'features: {}'\n%s", stdout)
	}
}

func TestInfraImportKeepsEmptyTopics(t *testing.T) {
	noTopics := `{"description":"d","visibility":"private","topics":[],"archived":false}`
	runner := importFake(map[string]string{targetGroupProj: noTopics})
	exit, stdout, _ := runWithRunner(runner, "infra", "import", targetGroupProj)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "topics: []") {
		t.Errorf("an empty topic list should be exported as 'topics: []'\n%s", stdout)
	}
}

func TestInfraImportSeparatesMultipleDocs(t *testing.T) {
	runner := importFake(map[string]string{
		"group/a": projJSON,
		"group/b": projJSON,
	})
	exit, stdout, _ := runWithRunner(runner, "infra", "import", "group/a", "group/b")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stdout, "\n---\n") {
		t.Errorf("multiple docs should be separated by ---\n%s", stdout)
	}
}

func TestInfraImportRequiresTarget(t *testing.T) {
	exit, _, _ := runWithRunner(&fakeInfraGlab{}, "infra", "import")
	if exit != 1 {
		t.Errorf("exit = %d, want 1 when no project is given", exit)
	}
}

func TestInfraImportContinuesPastMissing(t *testing.T) {
	runner := importFake(map[string]string{"group/ok": projJSON})
	exit, stdout, stderr := runWithRunner(runner, "infra", "import", "group/missing", "group/ok")

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a project is missing", exit)
	}
	if !strings.Contains(stdout, "name: ok") {
		t.Errorf("the existing project should still be exported\n%s", stdout)
	}
	if !strings.Contains(stderr, "missing") {
		t.Errorf("the missing project should be reported on stderr\n%s", stderr)
	}
}

func TestInfraImportRejectsMalformedTarget(t *testing.T) {
	for _, target := range []string{"/group/proj", "group//proj"} {
		t.Run(target, func(t *testing.T) {
			exit, _, stderr := runWithRunner(&fakeInfraGlab{}, "infra", "import", target)
			if exit != 1 {
				t.Errorf("exit = %d, want 1 for a malformed target", exit)
			}
			if !strings.Contains(stderr, "not in <owner/project> form") {
				t.Errorf("a malformed target should be reported as malformed, not fetched\n%s", stderr)
			}
		})
	}
}

// The emitted document carries every schedule the project owns, each attribute
// as GitLab reports it, ordered by description.
func TestInfraImportEmitsPipelineSchedules(t *testing.T) {
	runner := &fakeInfraGlab{
		projects: map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{targetGroupProj: `[
		  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *",
		   "cron_timezone":"UTC","active":true},
		  {"id":2,"description":"full ref","ref":"refs/tags/v1.0.0","cron":"0 5 * * *",
		   "cron_timezone":"Asia/Tokyo","active":false}]`},
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "import", targetGroupProj)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0\n%s", exit, stderr)
	}
	for _, want := range []string{
		"pipeline_schedules:",
		"description: full ref",
		"ref: refs/tags/v1.0.0",
		`cron: 0 5 * * *`,
		"cron_timezone: Asia/Tokyo",
		"active: false",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q\n%s", want, stdout)
		}
	}
	// Sorted by description, so "full ref" precedes "nightly" despite its id.
	if strings.Index(stdout, "full ref") > strings.Index(stdout, "description: nightly") {
		t.Errorf("schedules are not sorted by description\n%s", stdout)
	}
}

// A failed schedule read still exports the project's settings, with the
// schedules key left out and the cause reported on stderr.
func TestInfraImportExportsAProjectWhoseSchedulesCannotBeRead(t *testing.T) {
	runner := &fakeInfraGlab{
		projects:  map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{targetGroupProj: "not json"},
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "import", targetGroupProj)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a project is exported without its schedules", exit)
	}
	if !strings.Contains(stdout, "name: proj") {
		t.Errorf("the project's settings should still be exported\n%s", stdout)
	}
	if strings.Contains(stdout, "pipeline_schedules") {
		t.Errorf("an unread schedule list must not be declared in the document\n%s", stdout)
	}
	if !strings.Contains(stderr, "pipeline schedules") {
		t.Errorf("the failed schedule read should be reported on stderr\n%s", stderr)
	}
}

// The two ways a run falls short name different things to go and look at, so a
// run that hits both has to report both. Reporting only the missing project
// would leave the reader believing the emitted document is complete.
func TestInfraImportReportsBothAMissingProjectAndAnUnreadScheduleList(t *testing.T) {
	runner := &fakeInfraGlab{
		projects:  map[string]string{targetGroupProj: projJSON},
		schedules: map[string]string{targetGroupProj: "not json"},
	}

	exit, _, stderr := runWithRunner(runner, "infra", "import", "group/missing", targetGroupProj)

	if exit != 1 {
		t.Errorf("exit = %d, want 1", exit)
	}
	for _, want := range []string{"could not be imported", "without their pipeline schedules"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q, both shortfalls must be reported\n%s", want, stderr)
		}
	}
}

// A repeated description leaves the schedules undescribable, not the project.
// The repeat is reported so the reader can go and rename one, and both projects
// in the run are still exported.
func TestInfraImportReportsDuplicateSchedulesAndContinues(t *testing.T) {
	const otherProject = "group/other"
	runner := &fakeInfraGlab{
		projects: map[string]string{targetGroupProj: projJSON, otherProject: projJSON},
		schedules: map[string]string{targetGroupProj: `[
		  {"id":1,"description":"nightly","ref":"refs/heads/main","cron":"0 3 * * *"},
		  {"id":5,"description":"nightly","ref":"refs/heads/main","cron":"0 8 * * *"}]`},
	}

	exit, stdout, stderr := runWithRunner(runner, "infra", "import", targetGroupProj, otherProject)

	if exit != 1 {
		t.Errorf("exit = %d, want 1 when a project cannot be described", exit)
	}
	if !strings.Contains(stderr, "nightly") {
		t.Errorf("stderr missing the repeated description\n%s", stderr)
	}
	if !strings.Contains(stdout, "name: other") {
		t.Errorf("stdout missing the healthy project, later projects must still be emitted\n%s", stdout)
	}
	if !strings.Contains(stdout, "name: proj") {
		t.Errorf("the settings of the project with the repeated description are still describable\n%s", stdout)
	}
}

// The modeline up to the ref, which the dispatcher resolves from the build
// stamps: a release build stamps them in, so a test driving the dispatcher
// cannot name the ref it will see. The ref itself is asserted below, where the
// test supplies it.
const modelineHead = "# yaml-language-server: $schema=" +
	"https://raw.githubusercontent.com/Omochice/nyctereutes/"

// The rest of the modeline, which every ref shares.
const modelineTail = "/schema/repository.schema.json\n"

func TestInfraImportHeadsTheDocumentWithTheSchemaModeline(t *testing.T) {
	runner := importFake(map[string]string{targetGroupProj: projJSON})
	exit, stdout, _ := runWithRunner(runner, "infra", "import", targetGroupProj)

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if !strings.HasPrefix(stdout, modelineHead) {
		t.Errorf("stdout does not open with the schema modeline\n%s", stdout)
	}
	firstLine, _, _ := strings.Cut(stdout, "\n")
	if !strings.HasSuffix(firstLine+"\n", modelineTail) {
		t.Errorf("the modeline does not name the schema file, got %q", firstLine)
	}
}

// An editor reads the modeline out of a single document's leading comments, so
// writing it once would leave every document after the first unvalidated.
func TestInfraImportRepeatsTheSchemaModelineForEveryDocument(t *testing.T) {
	runner := importFake(map[string]string{
		"group/a": projJSON,
		"group/b": projJSON,
	})
	exit, stdout, _ := runWithRunner(runner, "infra", "import", "group/a", "group/b")

	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	if count := strings.Count(stdout, modelineHead); count != 2 {
		t.Errorf("the modeline appears %d times, want one per document\n%s", count, stdout)
	}
	if !strings.HasPrefix(stdout, modelineHead) {
		t.Errorf("the first document is not headed by the modeline\n%s", stdout)
	}
	if !strings.Contains(stdout, "---\n"+modelineHead) {
		t.Errorf("the second document is not headed by the modeline\n%s", stdout)
	}
}

// The schema an export points at is the one committed at the revision that
// wrote it, so the ref the tree is built with has to reach the output. The tree
// is built here rather than through the dispatcher, which derives the ref from
// the build stamps and so hands over a different one to a stamped build.
func TestInfraImportPointsAtTheSchemaOfTheGivenRef(t *testing.T) {
	stdout := &bytes.Buffer{}
	inout := &cli.ProcInout{
		Stdin:  strings.NewReader(""),
		Stdout: stdout,
		Stderr: io.Discard,
	}
	runner := importFake(map[string]string{targetGroupProj: projJSON})

	const ref = "refs/tags/v1.2.3"
	if err := infra.New(inout, runner, ref).Import.Execute(
		[]string{targetGroupProj},
	); err != nil {
		t.Fatalf("import error = %v", err)
	}

	want := modelineHead + ref + modelineTail
	if !strings.HasPrefix(stdout.String(), want) {
		t.Errorf("stdout does not open with %q\n%s", want, stdout.String())
	}
}
