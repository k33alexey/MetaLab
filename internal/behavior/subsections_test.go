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

// A subsystem that is no section keeps its place in the order written
// when the sections are moved (ML-STUDIO.md, «Подразделы», В6 of series
// 2.639): after the section it followed, first where it was first, last
// where the one it followed is gone; an order taken back keeps none.
// Defects caught: a hidden subsection dropped from the order, put at its
// start or end instead of after its section, or kept in an automatic order.
func TestAHiddenSubsectionKeepsItsPlace(t *testing.T) {
	a, b, c, hiddenFirst, hiddenAfterA, hiddenAfterGone, gone := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	hidden := func(id uuid.UUID) bool { return id == hiddenFirst || id == hiddenAfterA || id == hiddenAfterGone }
	said := []uuid.UUID{hiddenFirst, a, hiddenAfterA, b, gone, hiddenAfterGone, c}
	got := HiddenSubsectionsKept(said, []uuid.UUID{a, c, b}, hidden)
	if want := []uuid.UUID{hiddenFirst, a, hiddenAfterA, c, b, hiddenAfterGone}; !slices.Equal(got, want) {
		t.Errorf("kept %v, want %v", got, want)
	}
	if got := HiddenSubsectionsKept(said, nil, hidden); len(got) != 0 {
		t.Errorf("the automatic order kept %v", got)
	}
}
