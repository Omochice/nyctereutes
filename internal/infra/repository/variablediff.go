package repository

import (
	"cmp"
	"slices"

	"github.com/Omochice/nyctereutes/internal/infra/manifest"
)

// The variable work one schedule change implies. GitLab keeps a schedule's
// variables behind their own endpoints, so they are never carried by the
// schedule's own PUT and have to be reconciled key by key.
type variablePlan struct {
	added   []manifest.ScheduleVariable
	changed []manifest.ScheduleVariable
	removed []string
}

// Reports whether the plan asks for nothing.
func (plan variablePlan) empty() bool {
	return len(plan.added) == 0 && len(plan.changed) == 0 && len(plan.removed) == 0
}

// Works out which variables to add, change and remove. A nil declaration
// manages no variable and yields nothing; a declared list is the complete
// desired set, so a live key it does not name is removed. Order carries no
// meaning, so the sides are paired by key.
func planVariables(desired []manifest.ScheduleVariable, live []ScheduleVariable) variablePlan {
	var plan variablePlan
	if desired == nil {
		return plan
	}
	current := make(map[string]ScheduleVariable, len(live))
	for _, variable := range live {
		current[variable.Key] = variable
	}
	declared := make(map[string]struct{}, len(desired))
	for _, variable := range desired {
		declared[variable.Key] = struct{}{}
		existing, found := current[variable.Key]
		switch {
		case !found:
			plan.added = append(plan.added, variable)
		case !sameVariable(variable, existing):
			plan.changed = append(plan.changed, variable)
		}
	}
	for _, variable := range live {
		if _, kept := declared[variable.Key]; !kept {
			plan.removed = append(plan.removed, variable.Key)
		}
	}
	sortPlan(&plan)
	return plan
}

// Orders each part by key so a plan and the calls it drives do not depend on
// the order GitLab happened to report.
func sortPlan(plan *variablePlan) {
	byKey := func(left, right manifest.ScheduleVariable) int { return cmp.Compare(left.Key, right.Key) }
	slices.SortFunc(plan.added, byKey)
	slices.SortFunc(plan.changed, byKey)
	slices.Sort(plan.removed)
}

// Reports whether the live variable already matches the declared one.
func sameVariable(desired manifest.ScheduleVariable, live ScheduleVariable) bool {
	return desired.Value == string(live.Value) && string(desired.VariableType) == live.VariableType
}
