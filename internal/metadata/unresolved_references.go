package metadata

import (
	"slices"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// UnresolvedReference is a reference the prototype wrote down by identifier
// and nothing in the configuration carries: the object it pointed at was
// deleted and the reference was left behind. The prototype saves such a
// configuration and opens it - a form role falls back to the generated form, a
// command placed in the missing group is simply not drawn - so refusing it here
// would lose the whole configuration over one stale line. The configurations
// being moved have three: the variant form of one report in erp and the group
// of a command in two catalogs of sb.
//
// It is carried as written, behaves as not set, and is listed here for the
// import report, which is where a developer learns that it is there.
type UnresolvedReference struct {
	// Where names what holds the reference, as messages about it do.
	Where string
	ID    uuid.UUID
}

// UnresolvedReferences lists the references the last load found pointing at
// nothing, in the order they were met.
func (catalog *Catalog) UnresolvedReferences() []UnresolvedReference {
	return slices.Clone(catalog.unresolved)
}

func (catalog *Catalog) noteUnresolved(where string, id uuid.UUID) {
	catalog.unresolved = append(catalog.unresolved, UnresolvedReference{Where: where, ID: id})
}
