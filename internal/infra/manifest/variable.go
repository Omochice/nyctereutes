package manifest

import (
	"errors"
	"fmt"
)

// One variable a pipeline schedule passes to the pipelines it starts. GitLab
// cannot create a variable without a value, so declaring keys alone could only
// remove extras and report absences; the manifest carries the value, which
// makes keeping secrets out of these documents the operator's responsibility.
type ScheduleVariable struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
	// GitLab's variable_type: "env_var" passes the value as an environment
	// variable, "file" writes it to a file and passes the path.
	VariableType string `yaml:"variable_type"`
}

// GitLab reports a raw flag on a schedule variable but ignores it on both
// create and update, answering false whichever value is sent. A manifest field
// for it would be a promise the API does not keep: every plan would report the
// drift and every apply would try to fix it. It is left out until GitLab
// honours it.

// What GitLab stores when a variable is created without a type.
const defaultVariableType = "env_var"

// Decodes a variable, filling the attributes GitLab defaults on create. The
// defaults are seeded before decoding, so a declared value overwrites them.
func (variable *ScheduleVariable) UnmarshalYAML(unmarshal func(any) error) error {
	// The local type drops this method, so decoding into it does not recurse.
	type variableFields ScheduleVariable
	fields := variableFields{VariableType: defaultVariableType}
	if err := unmarshal(&fields); err != nil {
		return fmt.Errorf("decode schedule variable: %w", err)
	}
	*variable = ScheduleVariable(fields)
	return nil
}

// Signals two variables on one schedule sharing a key.
var errDuplicateVariable = errors.New("duplicate schedule variable key")

// Checks the variables of one schedule. The key is how the API addresses a
// variable, so a repeated one leaves an update or a delete undecidable and a
// blank one addresses nothing.
func validateVariables(schedule string, variables []ScheduleVariable) error {
	seen := make(map[string]struct{}, len(variables))
	for index, variable := range variables {
		if variable.Key == "" {
			return fmt.Errorf("%w: %s[%d].key", errRequiredField, variablePath(schedule), index)
		}
		if _, repeated := seen[variable.Key]; repeated {
			return fmt.Errorf("%w: %s uses %q twice", errDuplicateVariable, variablePath(schedule), variable.Key)
		}
		seen[variable.Key] = struct{}{}
	}
	return nil
}

// The manifest path of a schedule's variables, so an error points at the
// document rather than at an index alone.
func variablePath(schedule string) string {
	return fmt.Sprintf("spec.pipeline_schedules[%q].variables", schedule)
}
