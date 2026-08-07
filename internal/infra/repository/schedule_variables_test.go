package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Omochice/nyctereutes/internal/glab"
)

// Answers the single-schedule read with body, recording the args it was given.
func detailRunner(body string, record *[]string) glab.RunnerFunc {
	return func(_ context.Context, args ...string) ([]byte, error) {
		if record != nil {
			*record = args
		}
		return []byte(body), nil
	}
}

// The keys come back, and the read addresses one schedule rather than the list,
// which carries no variables.
func TestScheduleVariableKeysReadsOneScheduleAndReturnsItsKeys(t *testing.T) {
	var args []string
	body := `{"id":7,"description":"nightly","variables":[` +
		`{"key":"DEPLOY_TOKEN","value":"glpat-secret","variable_type":"env_var"},` +
		`{"key":"REGION","value":"tokyo","variable_type":"env_var"}]}`

	keys, err := NewClient(detailRunner(body, &args)).
		ScheduleVariableKeys(context.Background(), "group/sub", "proj", 7)
	if err != nil {
		t.Fatalf("ScheduleVariableKeys: %v", err)
	}

	if want := []string{"DEPLOY_TOKEN", "REGION"}; !slices.Equal(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}
	if want := "projects/group%2Fsub%2Fproj/pipeline_schedules/7"; !slices.Contains(args, want) {
		t.Errorf("args = %v, want the schedule endpoint %q", args, want)
	}
}

// No value reaches the caller, which is the property that lets a plan render
// what it is given. A key alone cannot disclose a credential.
func TestScheduleVariableKeysReturnsNoValue(t *testing.T) {
	const secret = "glpat-secret"
	body := `{"variables":[{"key":"DEPLOY_TOKEN","value":"` + secret + `","variable_type":"env_var"}]}`

	keys, err := NewClient(detailRunner(body, nil)).
		ScheduleVariableKeys(context.Background(), ownerGroup, nameProj, 7)
	if err != nil {
		t.Fatalf("ScheduleVariableKeys: %v", err)
	}
	for _, key := range keys {
		if strings.Contains(key, secret) {
			t.Errorf("keys = %v, want no value carried", keys)
		}
	}
}

// A schedule that genuinely holds no variable reads as an empty set rather than
// a failure, so its removal is shown with nothing listed under it.
func TestScheduleVariableKeysReadsAnEmptyListAsNoVariables(t *testing.T) {
	keys, err := NewClient(detailRunner(`{"id":7,"variables":[]}`, nil)).
		ScheduleVariableKeys(context.Background(), ownerGroup, nameProj, 7)
	if err != nil {
		t.Fatalf("ScheduleVariableKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("keys = %v, want none", keys)
	}
}

// GitLab answers 200 with the variables field left out for a reader who may not
// see them, so an absent field is reported rather than read as a schedule
// holding none: the two mean opposite things to someone approving a removal.
func TestScheduleVariableKeysReportsAnUndisclosedSet(t *testing.T) {
	for name, body := range map[string]string{
		"field absent": `{"id":7,"description":"nightly"}`,
		"field null":   `{"id":7,"variables":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewClient(detailRunner(body, nil)).
				ScheduleVariableKeys(context.Background(), ownerGroup, nameProj, 7)
			if !errors.Is(err, errVariablesNotDisclosed) {
				t.Errorf("error = %v, want it to wrap errVariablesNotDisclosed", err)
			}
		})
	}
}
