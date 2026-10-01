package project

import (
	"fmt"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// InterfaceVisibility is whether one item of the command interface is shown:
// to everybody or to nobody, unless a role says otherwise. It is the shape the
// prototype gives every visibility of its command interface - a command in a
// section, a section in the application - so it is described once.
//
// Carried, not yet acted upon: the command interface is built in block 18.
type InterfaceVisibility struct {
	// Common is the answer for a role that says nothing. It is written out even
	// when false, because false is the more common answer for a command hidden
	// from a section and showing it to the roles listed below.
	Common bool `yaml:"common" json:"common"`
	// Roles are the exceptions, in the order the configuration lists them.
	Roles []RoleVisibility `yaml:"roles,omitempty" json:"roles,omitempty"`
}

// RoleVisibility is one exception: this role sees the item, or does not.
type RoleVisibility struct {
	Role    uuid.UUID `yaml:"role" json:"role"`
	Visible bool      `yaml:"visible" json:"visible"`
}

// SubsystemVisibility is whether one section of the application is shown.
type SubsystemVisibility struct {
	Subsystem           uuid.UUID `yaml:"subsystem" json:"subsystem"`
	InterfaceVisibility `yaml:",inline"`
}

// ValidateInterfaceVisibility checks a visibility by its shape: every role
// named once and by a real identifier. Whether the role exists is known only
// where the whole configuration is.
func ValidateInterfaceVisibility(path string, value InterfaceVisibility) []string {
	var issues []string
	seen := make(map[uuid.UUID]bool, len(value.Roles))
	for index, role := range value.Roles {
		prefix := fmt.Sprintf("%s.roles[%d]", path, index)
		if role.Role.IsZero() {
			issues = append(issues, prefix+".role must be a non-zero UUID")
			continue
		}
		if seen[role.Role] {
			issues = append(issues, prefix+".role must be unique")
		}
		seen[role.Role] = true
	}
	return issues
}

// validateSubsystemsInterface checks the root's half of the command interface
// by its shape: each section ordered once and given one visibility.
func validateSubsystemsInterface(order []uuid.UUID, visibility []SubsystemVisibility) []string {
	var issues []string
	ordered := make(map[uuid.UUID]bool, len(order))
	for index, id := range order {
		prefix := fmt.Sprintf("subsystems_order[%d]", index)
		if id.IsZero() {
			issues = append(issues, prefix+" must be a non-zero UUID")
			continue
		}
		if ordered[id] {
			issues = append(issues, prefix+" must be unique")
		}
		ordered[id] = true
	}
	seen := make(map[uuid.UUID]bool, len(visibility))
	for index, item := range visibility {
		prefix := fmt.Sprintf("subsystems_visibility[%d]", index)
		if item.Subsystem.IsZero() {
			issues = append(issues, prefix+".subsystem must be a non-zero UUID")
		} else if seen[item.Subsystem] {
			issues = append(issues, prefix+".subsystem must be unique")
		}
		seen[item.Subsystem] = true
		issues = append(issues, ValidateInterfaceVisibility(prefix, item.InterfaceVisibility)...)
	}
	return issues
}
