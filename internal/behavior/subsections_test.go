package behavior

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The automatic order of «Подразделы» is the alphabet of the presentations
// in Russian (podsistema.md, «Кнопки и меню»; snapshots vopr-2.636/20-22),
// written out here: a small letter among the capital ones, not after them as
// the bytes would put it; two of one presentation as they came. A
// subsystem added goes last - after the order said, or after the automatic
// order of the others where none is said - and once. Defects caught: the
// order of the bytes, «Яблоки» before «апельсины»; the new one put by the
// alphabet or twice; a said order replaced.
func TestTheSubsectionsAreOrderedAsThePrototypes(t *testing.T) {
	titles := []string{"Яблоки", "апельсины", "Бананы", "Ёжики", "бананы", "Груши"}
	AutomaticSubsections(titles, func(title string) string { return title })
	if want := []string{"апельсины", "бананы", "Бананы", "Груши", "Ёжики", "Яблоки"}; !slices.Equal(titles, want) {
		t.Errorf("ordered %q, want %q", titles, want)
	}
	one, two, three, added := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	for name, testCase := range map[string]struct{ said, automatic, want []uuid.UUID }{
		"none said":             {nil, []uuid.UUID{two, one, added}, []uuid.UUID{two, one, added}},
		"said":                  {[]uuid.UUID{three, one}, []uuid.UUID{one, two, three, added}, []uuid.UUID{three, one, added}},
		"said with the new one": {[]uuid.UUID{added, one}, []uuid.UUID{one, added}, []uuid.UUID{added, one}},
	} {
		if got := SubsectionAdded(testCase.said, testCase.automatic, added); !slices.Equal(got, testCase.want) {
			t.Errorf("%s: %v, want %v", name, got, testCase.want)
		}
	}
}
