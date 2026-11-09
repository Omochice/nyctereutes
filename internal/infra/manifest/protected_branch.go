package manifest

import (
	"fmt"

	"github.com/invopop/jsonschema"
)

// One protected branch a project owns.
// Unlike [PipelineSchedule], it carries no server-assigned id: GitLab addresses a protection by its name.
type ProtectedBranch struct {
	Name string `yaml:"name" jsonschema:"required"`
	// All three lack omitempty yet are optional, as on [PipelineSchedule]: UnmarshalYAML supplies a default for each.
	PushAccessLevel  BranchAccessLevel `yaml:"push_access_level"`
	MergeAccessLevel BranchAccessLevel `yaml:"merge_access_level"`
	AllowForcePush   bool              `yaml:"allow_force_push"`
	// Nil rather than a default, because only an Enterprise build reports it: nil leaves whatever the project holds alone.
	//
	// code_owner_approval_required is not a key.
	// An Enterprise build reports it but refuses the value without a licence, so it cannot be reconciled.
	UnprotectAccessLevel *BranchUnprotectAccessLevel `yaml:"unprotect_access_level,omitempty"`
}

// What GitLab grants when a branch is protected without an access level given.
const defaultBranchAccessLevel = BranchAccessLevel(valueMaintainer)

// Decodes a protected branch through the surrounding decoder, for the reason [PipelineSchedule.UnmarshalYAML] gives.
func (branch *ProtectedBranch) UnmarshalYAML(unmarshal func(any) error) error {
	// The local type drops this method, so decoding into it does not recurse.
	type branchFields ProtectedBranch
	fields := branchFields{
		PushAccessLevel:  defaultBranchAccessLevel,
		MergeAccessLevel: defaultBranchAccessLevel,
	}
	if err := unmarshal(&fields); err != nil {
		return fmt.Errorf("decode protected branch: %w", err)
	}
	*branch = ProtectedBranch(fields)
	return nil
}

func (branch *ProtectedBranch) validate(index int) error {
	if branch.Name == "" {
		return fmt.Errorf("%w: spec.protected_branches[%d].%s", errRequiredField, index, fieldName)
	}
	return nil
}

const (
	valueNoOne      = "no_one"
	valueDeveloper  = "developer"
	valueMaintainer = "maintainer"
	valueAdmin      = "admin"
)

// Who may push to or merge into a protected branch: "no_one", "developer", "maintainer" or "admin".
// Kept apart from [AccessLevel], which says how far a feature is opened up, not which role an operation demands.
type BranchAccessLevel string

// Rejects values outside the branch access-level set at decode time.
func (level *BranchAccessLevel) UnmarshalYAML(data []byte) error {
	value, err := enumValue(data, "branch access level", level.allowedValues())
	if err != nil {
		return err
	}
	*level = BranchAccessLevel(value)
	return nil
}

// Reports the branch access levels to the schema generator, which cannot see through UnmarshalYAML.
func (level BranchAccessLevel) JSONSchema() *jsonschema.Schema {
	return enumSchema(level.allowedValues())
}

func (*BranchAccessLevel) allowedValues() []string {
	return []string{valueNoOne, valueDeveloper, valueMaintainer, valueAdmin}
}

// Who may lift a branch's protection: "developer", "maintainer" or "admin".
// Kept apart from [BranchAccessLevel] because "no_one" is absent: GitLab answers an unprotect level of 0 with 400.
type BranchUnprotectAccessLevel string

// Rejects values outside the unprotect access-level set at decode time.
func (level *BranchUnprotectAccessLevel) UnmarshalYAML(data []byte) error {
	value, err := enumValue(data, "unprotect access level", level.allowedValues())
	if err != nil {
		return err
	}
	*level = BranchUnprotectAccessLevel(value)
	return nil
}

// Reports the unprotect access levels to the schema generator, which cannot see through UnmarshalYAML.
func (level BranchUnprotectAccessLevel) JSONSchema() *jsonschema.Schema {
	return enumSchema(level.allowedValues())
}

func (*BranchUnprotectAccessLevel) allowedValues() []string {
	return []string{valueDeveloper, valueMaintainer, valueAdmin}
}
