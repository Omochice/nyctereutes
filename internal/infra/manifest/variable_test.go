package manifest

import (
	"strings"
	"testing"
)

const variableDoc = `apiVersion: nyctereutes/v1
kind: Repository
metadata:
  name: proj
  owner: group
spec:
  topics: []
  pipeline_schedules:
    - description: nightly
      ref: main
      cron: "0 3 * * *"
      variables:
        - key: DEPLOY_ENV
          value: staging
        - key: CONFIG
          value: |-
            line one
            line two
          variable_type: file
`

// A variable cannot be created without a value, so declaring keys alone could
// only delete extras and report absences. The manifest therefore carries the
// value, and the omitted attributes take GitLab's create defaults.
func TestParseReadsScheduleVariables(t *testing.T) {
	repos, errs := Parse([]byte(variableDoc))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}

	variables := repos[0].Spec.PipelineSchedules[0].Variables
	if len(variables) != 2 {
		t.Fatalf("variables = %+v, want 2", variables)
	}
	deploy := ScheduleVariable{Key: "DEPLOY_ENV", Value: "staging", VariableType: "env_var"}
	if variables[0] != deploy {
		t.Errorf("variable = %+v, want %+v", variables[0], deploy)
	}
	config := ScheduleVariable{Key: "CONFIG", Value: "line one\nline two", VariableType: "file"}
	if variables[1] != config {
		t.Errorf("variable = %+v, want %+v", variables[1], config)
	}
}

// An omitted block leaves the live variables alone while an empty list declares
// that the schedule should carry none.
func TestParseDistinguishesOmittedVariablesFromAnEmptyList(t *testing.T) {
	omitted, errs := Parse(scheduleDocWithRef("main"))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if got := omitted[0].Spec.PipelineSchedules[0].Variables; got != nil {
		t.Errorf("omitted block = %v, want nil (unmanaged)", got)
	}

	declared, errs := Parse([]byte(validDoc + `  pipeline_schedules:
    - description: nightly
      ref: main
      cron: "0 3 * * *"
      variables: []
`))
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if got := declared[0].Spec.PipelineSchedules[0].Variables; got == nil || len(got) != 0 {
		t.Errorf("empty list = %v, want an empty slice", got)
	}
}

// The key addresses a variable in the API, so a repeated one leaves an update
// or a delete undecidable, and a blank one addresses nothing.
func TestParseRejectsUnusableVariableKeys(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "repeated key",
			body: "        - key: DEPLOY_ENV\n          value: a\n        - key: DEPLOY_ENV\n          value: b\n",
			want: "DEPLOY_ENV",
		},
		{
			name: "blank key",
			body: "        - key: \"\"\n          value: a\n",
			want: "key",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			doc := validDoc + `  pipeline_schedules:
    - description: nightly
      ref: main
      cron: "0 3 * * *"
      variables:
` + testCase.body
			_, errs := Parse([]byte(doc))
			if len(errs) != 1 {
				t.Fatalf("errs = %v, want one", errs)
			}
			if !strings.Contains(errs[0].Error(), testCase.want) {
				t.Errorf("error = %q, want it to name %q", errs[0], testCase.want)
			}
		})
	}
}

// A schedule declared to carry no variable has to say so, for the reason the
// schedule list itself does: an omitted block and an empty one mean different
// things.
func TestMarshalKeepsAnEmptyVariableList(t *testing.T) {
	doc := fullRepository()
	doc.Spec.PipelineSchedules[0].Variables = []ScheduleVariable{}

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "variables: []") {
		t.Errorf("emitted document does not declare an empty variable list:\n%s", out)
	}
}

// A schedule whose variables were never read says nothing about them, so a
// command that has not paid for the per-schedule request does not describe the
// schedule as carrying none.
func TestMarshalOmitsUndeclaredVariables(t *testing.T) {
	doc := fullRepository()
	doc.Spec.PipelineSchedules[0].Variables = nil

	out, err := Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "variables") {
		t.Errorf("emitted document declares variables the state never described:\n%s", out)
	}

	repos, errs := Parse(out)
	if len(errs) > 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if variables := repos[0].Spec.PipelineSchedules[0].Variables; variables != nil {
		t.Errorf("variables = %v after a round trip, want nil", variables)
	}
}

// GitLab accepts only env_var and file, so a typo is caught locally with both
// spellings named rather than deferred to a rejection from the API.
func TestParseRejectsUnknownVariableType(t *testing.T) {
	doc := validDoc + `  pipeline_schedules:
    - description: nightly
      ref: main
      cron: "0 3 * * *"
      variables:
        - key: A
          value: b
          variable_type: secret
`
	_, errs := Parse([]byte(doc))
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want one", errs)
	}
	if !strings.Contains(errs[0].Error(), "env_var, file") {
		t.Errorf("error = %q, want it to list the allowed values", errs[0])
	}
}
