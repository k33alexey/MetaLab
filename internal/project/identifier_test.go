package project

import "testing"

// The configuration's own name, a language's name and the name prefix follow
// the same rule as a metadata object: an underscore counts as a letter.
//
// Defect caught: the project rule drifting from the metadata one, so that a
// configuration named with an underscore is refused while its objects are not.
func TestProjectIdentifierTreatsUnderscoreAsALetter(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{
		"_Конфигурация": true, "Бух_Учёт": true, "Имя1": true,
		"1Имя": false, "Имя-1": false, "": false,
	} {
		if got := isIdentifier(name); got != want {
			t.Errorf("isIdentifier(%q) = %v, want %v", name, got, want)
		}
	}
}
