package manifest

import (
	"fmt"

	"github.com/invopop/jsonschema"
)

// One protected branch a project owns. Unlike [PipelineSchedule], whose
// server-assigned id has to be kept out of the document, GitLab addresses a
// protected branch by the name the manifest already holds, so the name is both
// the identity and the address.
//
// The name is a branch name or a wildcard pattern ("release/*"), which GitLab
// accepts alike and does not check against the repository, so it is held
// verbatim rather than canonicalized the way [Ref] is.
type ProtectedBranch struct {
	Name string `yaml:"name" jsonschema:"required"`
	// All three are optional despite lacking omitempty, for the reason the same
	// fields on [PipelineSchedule] give: the emitter always writes them, but
	// UnmarshalYAML supplies a default for each.
	//
	// GitLab CE reports no unprotect access level and no code-owner flag, both
	// being Premium features it accepts on create and never answers with. A
	// setting that can be written but not read cannot be reconciled, so neither
	// is a key here.
	PushAccessLevel  BranchAccessLevel `yaml:"push_access_level"`
	MergeAccessLevel BranchAccessLevel `yaml:"merge_access_level"`
	AllowForcePush   bool              `yaml:"allow_force_push"`
}

// What GitLab grants when a branch is protected without an access level given.
const defaultBranchAccessLevel = BranchAccessLevel(valueMaintainer)

// Decodes a protected branch, filling the attributes GitLab defaults on create,
// seeded before decoding so a declared value overwrites them.
//
// Decoding runs through the callback for the reason [PipelineSchedule] does:
// the surrounding decoder's strictness applies inside the branch too, and its
// errors point at the line in the file.
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

// Reports a branch left unnamed, locating it by its position in the document.
// The name is what GitLab addresses the protection by, so an unnamed one could
// neither be created nor paired with a live branch.
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

// Who may push to or merge into a protected branch: "no_one", "developer",
// "maintainer" or "admin". Each names a floor rather than an exact role, which
// is why GitLab renders "developer" as "Developers + Maintainers".
//
// A separate type from [AccessLevel], which answers a different question: that
// one says how far a project feature is opened up, this one says which role a
// branch operation demands.
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

// Reports the branch access levels to the schema generator, which cannot see
// through UnmarshalYAML.
func (level BranchAccessLevel) JSONSchema() *jsonschema.Schema {
	return enumSchema(level.allowedValues())
}

func (*BranchAccessLevel) allowedValues() []string {
	return []string{valueNoOne, valueDeveloper, valueMaintainer, valueAdmin}
}
