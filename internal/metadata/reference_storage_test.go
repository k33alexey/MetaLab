package metadata

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestReferencesSayHowTheyAreStored catches a reference stored the way
// another is: a reference to a document given no foreign key, an enumeration
// value given one to a table that does not exist, a point of a route put
// into a uuid column, a record of an external source given a column at all.
// It also catches a reference left out of the descriptor's storage, which
// every path below would then refuse. The expectation is written out here
// from the schema the infobase has, not read from kindInfos.
func TestReferencesSayHowTheyAreStored(t *testing.T) {
	t.Parallel()
	expected := map[TypeKind]referenceStorage{
		CatalogType: tableRow, DocumentType: tableRow, CharacteristicTypesType: tableRow, AccountType: tableRow,
		CalculationTypeType: tableRow, BusinessProcessType: tableRow, TaskType: tableRow, ExchangePlanType: tableRow,
		EnumerationType: listedValue, RoutePointType: byName,
		ExternalTableType: notStored, ExternalDimensionTableType: notStored,
	}
	if !maps.Equal(referenceStorages, expected) {
		t.Errorf("references are stored as %v, expected %v", referenceStorages, expected)
	}
	for _, item := range kindInfos {
		for _, reference := range item.references {
			if reference.stored == storageUnsaid {
				t.Errorf("%s of %s says nothing of its storage", reference.kind, item.kind)
			}
		}
	}
	empty := &Catalog{}
	for kind, stored := range expected {
		id := uuid.MustNew()
		// Only a row of a table is checked as an object reference; the rest
		// have their own checks, and a record of an external source is no
		// value of the infobase.
		_, ok, reason := empty.normalizeAs(Value{Kind: kind, Data: uuid.UUID{}.String()}, Type{Kind: kind, Reference: &id})
		if ok || (reason == "object reference cannot be empty") != (stored == tableRow) {
			t.Errorf("%s normalized from an empty reference: %t, %q", kind, ok, reason)
		}
		storage, err := empty.attributeStorage([]Type{{Kind: kind, Reference: &id}})
		if stored == notStored {
			if err == nil || !storedNowhere(kind) {
				t.Errorf("%s is given a column %q", kind, storage.sqlType)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", kind, err)
			continue
		}
		column, cast := "uuid", "::uuid"
		if stored == byName {
			column, cast = "character varying(128)", "::text"
		}
		if storage.sqlType != column || storage.valueType != kind || storage.composite {
			t.Errorf("%s is stored as %+v, expected column %s", kind, storage, column)
		}
		if foreign := storage.referenceObject != nil && *storage.referenceObject == id; foreign != (stored == tableRow) {
			t.Errorf("%s refers to its table by a foreign key: %t", kind, foreign)
		}
		if got := queryStorageCast(storage); got != cast {
			t.Errorf("%s is cast in a query as %s, expected %s", kind, got, cast)
		}
		if got := temporarySQLType(queryColumn{storage: attributeStorage{valueType: kind}}); got != strings.TrimPrefix(cast, "::") {
			t.Errorf("%s takes a column %s of a temporary table", kind, got)
		}
		data := uuid.MustNew().String()
		written, err := databaseAttributeValue(storage, Value{Kind: kind, Data: data})
		if err != nil || written != data {
			t.Errorf("%s is written as %v (%v)", kind, written, err)
		}
		raw, _ := json.Marshal(data)
		read, err := decodeDatabaseAttribute(storage, raw)
		if err != nil || read.Data != data || read.Kind != kind || (stored == tableRow) != (read.Object == id) {
			t.Errorf("%s is read back as %+v (%v)", kind, read, err)
		}
		// A reference takes no qualifier and is not refused as unknown.
		issues := validateTypes("types", []Type{{Kind: kind, Reference: &id, Length: 10}})
		if len(issues) != 1 || !strings.HasSuffix(issues[0], "has unsupported qualifiers") {
			t.Errorf("%s with a length: %v", kind, issues)
		}
	}
}
