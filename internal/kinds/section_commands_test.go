package kinds

import (
	"slices"
	"testing"
)

// Every kind says which standard commands its objects give the section of a
// subsystem, or that they give none - once. Defect caught: a kind added
// with no word on its commands, which the window «Командный интерфейс»
// would then silently leave empty.
func TestEveryKindSaysItsSectionCommands(t *testing.T) {
	for _, descriptor := range All() {
		_, gives := sectionCommands[descriptor.Kind]
		none := slices.Contains(withoutSectionCommands, descriptor.Kind)
		if gives == none {
			t.Errorf("%s: gives commands %v, gives none %v", descriptor.Kind, gives, none)
		}
	}
}
