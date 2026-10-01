package metadata

import (
	"fmt"
	"slices"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Ввод на основании - making a new object out of one the user is already
// looking at: an invoice out of a delivery, an assignment out of a file. The
// object being made names what it may be made out of, and that list is the
// whole of the mechanism's data.
//
// Filling the new object is not here. That happens in the ОбработкаЗаполнения
// event of the object module, which is block 9; what this iteration carries is
// the list, without which the command that calls the event is never offered at
// all.
//
// The carriers are the eight kinds that have an object of their own to make -
// a catalog, a document, a chart of characteristic types, a chart of accounts,
// a chart of calculation types, a business process, a task, an exchange plan.
// The syntax assistant gives ВводитсяНаОсновании to those eight and to one
// more, the table of an external data source, whose properties are deliberately
// not unfolded yet. The demonstration configuration declares the property on
// all eight and fills it on fourteen objects.
//
// The tab of the editing window shows two lists, and only the upper one is
// stored here. The lower one - "objects this one is a basis for" - is the same
// links read backwards, and editing it edits the other objects' lists. Keeping
// it as a second property would be keeping the same fact twice, and the two
// copies would disagree the first time an object was deleted; see BasisFor,
// which answers it by reading rather than by remembering. The requirements said
// a document stores both lists, and that is corrected there.

// BasisObject is one object a new one may be made out of, resolved against the
// configuration.
type BasisObject struct {
	Kind Kind
	ID   uuid.UUID
	Name string
}

// basisCarrier is one object that carries a list of bases, so that the eight
// kinds can be walked in one loop. The name is what a message calls the object;
// the list is the object's own, not a copy.
type basisCarrier struct {
	kind    Kind
	id      uuid.UUID
	name    string
	basedOn []uuid.UUID
}

// basisCarriers walks the eight kinds that may be entered on the basis of
// something. The order is the order of the kinds in the tree, so that a message
// about a configuration does not change its wording between two loads.
func (catalog *Catalog) basisCarriers() []basisCarrier {
	var result []basisCarrier
	for _, item := range catalog.Catalogs {
		result = append(result, basisCarrier{CatalogKind, item.ID, item.Name, item.BasedOn})
	}
	for _, item := range catalog.Documents {
		result = append(result, basisCarrier{DocumentKind, item.ID, item.Name, item.BasedOn})
	}
	for _, item := range catalog.ChartsOfCharacteristicTypes {
		result = append(result, basisCarrier{ChartOfCharacteristicTypesKind, item.ID, item.Name, item.BasedOn})
	}
	for _, item := range catalog.ChartsOfAccounts {
		result = append(result, basisCarrier{ChartOfAccountsKind, item.ID, item.Name, item.BasedOn})
	}
	for _, item := range catalog.ChartsOfCalculationTypes {
		result = append(result, basisCarrier{ChartOfCalculationTypesKind, item.ID, item.Name, item.BasedOn})
	}
	for _, item := range catalog.BusinessProcesses {
		result = append(result, basisCarrier{BusinessProcessKind, item.ID, item.Name, item.BasedOn})
	}
	for _, item := range catalog.Tasks {
		result = append(result, basisCarrier{TaskKind, item.ID, item.Name, item.BasedOn})
	}
	for _, item := range catalog.ExchangePlans {
		result = append(result, basisCarrier{ExchangePlanKind, item.ID, item.Name, item.BasedOn})
	}
	// A table of a source is entered on the basis of other objects and may be
	// the basis of one, the help says - but only a table of object data: a
	// basis is handed to the filling event as the object it is, and a record
	// is not one.
	for _, source := range catalog.ExternalDataSources {
		for _, table := range source.Tables {
			if table.ObjectTable() {
				result = append(result, basisCarrier{ExternalDataSourceTableKind, table.ID, source.Name + "." + table.Name, table.BasedOn})
			}
		}
	}
	return result
}

// basisObject finds one object by identifier among the kinds that may be a
// basis. The second result is false when nothing of the configuration has that
// identifier.
//
// Which kinds those are is the question the help answers too narrowly. The page
// about editing a document says the two lists hold catalogs and documents; the
// demonstration configuration has a business process entered on the basis of a
// task, so the sentence describes one kind's dialog rather than the mechanism.
// The answer taken here is the same eight kinds that may carry a list, and for
// the same reason: a basis is passed to the filling event as the object it is,
// so it has to be an object somebody can have open.
func (catalog *Catalog) basisObject(id uuid.UUID) (BasisObject, bool) {
	for _, carrier := range catalog.basisCarriers() {
		if carrier.id == id {
			return BasisObject{Kind: carrier.kind, ID: carrier.id, Name: carrier.name}, true
		}
	}
	return BasisObject{}, false
}

// EnteredOnBasisOf are the objects one object may be made out of, in the order
// it names them. An object that is not in the configuration is left out rather
// than guessed at; loading refuses such a configuration, so this can only
// happen to a catalog assembled by hand.
func (catalog *Catalog) EnteredOnBasisOf(object uuid.UUID) []BasisObject {
	var result []BasisObject
	for _, carrier := range catalog.basisCarriers() {
		if carrier.id != object {
			continue
		}
		for _, id := range carrier.basedOn {
			if basis, ok := catalog.basisObject(id); ok {
				result = append(result, basis)
			}
		}
		break
	}
	return result
}

// BasisFor is the other list of the tab: what may be made out of this object.
// It is the same links read backwards, and it is read every time rather than
// kept beside them - a second copy is a thing that can disagree with the first.
// The walk is over the objects of a configuration, which is the same walk the
// caller is about to make to build the menu it is asking about.
//
// An object may be a basis for itself, and three documents of the
// demonstration configuration say so: a call is entered on the basis of a call.
// So this answers with the object itself when the object names itself.
func (catalog *Catalog) BasisFor(object uuid.UUID) []BasisObject {
	var result []BasisObject
	for _, carrier := range catalog.basisCarriers() {
		if slices.Contains(carrier.basedOn, object) {
			result = append(result, BasisObject{Kind: carrier.kind, ID: carrier.id, Name: carrier.name})
		}
	}
	return result
}

// validateBasedOn is the shape of the list, without the configuration: how long
// it is, and whether it names the same object twice. Naming an object twice
// would offer the same command twice.
func validateBasedOn(basedOn []uuid.UUID) []string {
	return validateUniqueIDs("based_on", basedOn)
}

// validateBasedOnReferences resolves every list against the configuration. A
// basis that is not there is refused rather than dropped: the command to make
// this object out of it would simply never appear, and nobody would be told
// why.
func (catalog *Catalog) validateBasedOnReferences() error {
	for _, carrier := range catalog.basisCarriers() {
		for index, id := range carrier.basedOn {
			if _, ok := catalog.basisObject(id); !ok {
				return fmt.Errorf("%s %s is entered on the basis of %s (based_on[%d]), which is not an object of the configuration that can be a basis",
					carrier.kind, carrier.name, id, index)
			}
		}
	}
	return nil
}
