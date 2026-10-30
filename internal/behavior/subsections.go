package behavior

import (
	"slices"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// «Подразделы» of a section (docs/block-00/podsistema.md, «Кнопки и
// меню», «Досъёмка 28.10.2026, ночь»; snapshots vopr-2.636/20-22,
// dosyomka-2639/01): the subsystems under a subsystem - or the sections of
// the root - stand in the order its command interface says
// (SubsystemsOrder), the rest after them by the alphabet of their
// presentations, Russian - the automatic order.

// russian orders the presentations of the subsections as the prototype's
// list does.
var russian = collate.New(language.Russian)

// AutomaticSubsections orders subsections by the alphabet of their
// presentations, title(item): the automatic order of «Подразделы». Two of
// one presentation keep the order they came in.
func AutomaticSubsections[T any](subsections []T, title func(T) string) {
	slices.SortStableFunc(subsections, func(one, other T) int { return russian.CompareString(title(one), title(other)) })
}

// SubsectionAdded is the order a subsystem's command interface says of its
// subsections after a subsystem was made under it, added: the order said,
// or where it says none the automatic one of those that were there,
// automatic, and the new one last - the prototype's parent shows
// «Подразделы (ручной порядок)» at once (dosyomka-2639/01).
func SubsectionAdded(said, automatic []uuid.UUID, added uuid.UUID) []uuid.UUID {
	order := slices.Clone(said)
	if len(order) == 0 {
		order = slices.DeleteFunc(slices.Clone(automatic), func(id uuid.UUID) bool { return id == added })
	}
	if !slices.Contains(order, added) {
		order = append(order, added)
	}
	return order
}
