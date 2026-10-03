package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Where each new kind of type may stand. The object of a report or a data
// processor and the manager of a record live in memory: a running object and a
// defined type, not a stored field or a command. A type of the platform by
// name likewise. A reference to a table of an external source and the type of
// a vanished object are stored nowhere: a running object, a command parameter
// and a defined type take them, a stored field does not.
//
// Defect caught: a kind refused where the prototype keeps it (energy's report
// objects and filters, the vanished type of a command of erp, a reference to a
// source in a command), and a kind let into a stored field, which has no
// column to put it in.
func TestEachNewKindOfTypeStandsWhereItMay(t *testing.T) {
	t.Parallel()
	id := uuid.MustNew()
	places := []struct {
		name  string
		place typePlace
	}{{"stored", placeStored}, {"running", placeRunningObject}, {"defined", placeDefinedType}, {"command", placeCommandParameter}, {"session", placeSessionParameter}}
	for _, test := range []struct {
		item    Type
		allowed map[string]bool
	}{
		{Type{Kind: ReportObjectType, Reference: &id}, map[string]bool{"running": true, "defined": true}},
		{Type{Kind: DataProcessorObjectType, Reference: &id}, map[string]bool{"running": true, "defined": true}},
		{Type{Kind: InformationRegisterRecordManagerType, Reference: &id}, map[string]bool{"running": true, "defined": true}},
		{Type{Kind: PlatformType, Name: "cfg:Filter"}, map[string]bool{"running": true, "defined": true}},
		{Type{Kind: ExternalTableType, Reference: &id}, map[string]bool{"running": true, "defined": true, "command": true}},
		{Type{Kind: VanishedType, Reference: &id}, map[string]bool{"running": true, "defined": true, "command": true}},
	} {
		for _, place := range places {
			issues := validateTypesIn("types", []Type{test.item}, uuid.UUID{}, place.place)
			if test.allowed[place.name] && len(issues) != 0 {
				t.Errorf("%s in %s was refused: %v", test.item.Kind, place.name, issues)
			}
			if !test.allowed[place.name] && len(issues) == 0 {
				t.Errorf("%s in %s was accepted", test.item.Kind, place.name)
			}
		}
	}
	if issues := validateTypesIn("types", []Type{{Kind: PlatformType}}, uuid.UUID{}, placeRunningObject); len(issues) == 0 {
		t.Error("a platform type with no name was accepted")
	}
	if issues := validateTypesIn("types", []Type{{Kind: StringType, Length: 10, Name: "x"}}, uuid.UUID{}, placeStored); len(issues) == 0 {
		t.Error("a name on a type that is not of the platform was accepted")
	}
}

// The parameter of a command is resolved against the configuration, as the
// types of an attribute are; the type of a vanished object is not resolved.
// And a defined type that brings a reference to a source into a stored field
// is refused when the schema is built.
//
// Defect caught: a command typed by a catalog that is not there, offered
// beside nothing; a vanished type refused for naming nothing, which is what it
// is; and a stored field given a type with no column behind it.
func TestCommandParametersResolveAndStoredFieldsRefuseWhatIsStoredNowhere(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	noteCatalog(t, root, "commands:\n  - {id: "+uuid.MustNew().String()+", name: Подбор, title: {ru: Подбор}, parameter: [{kind: catalog, reference: "+uuid.MustNew().String()+"}]}\n", "")
	writeCommandModule(t, root, CatalogKind, "Товары", "Подбор")
	if message := loadRefused(t, root, "a command typed by a catalog that is not there"); !strings.Contains(message, "references unknown catalog") {
		t.Fatalf("the error does not say why: %v", message)
	}

	root = metadataProject(t)
	writeExternalSource(t, root, warehouseSource, "Склад", "")
	writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody)
	defined := uuid.MustNew().String()
	writeMetadata(t, root, DefinedTypeKind, defined, "format: 1\nid: "+defined+"\nname: ТоварСклада\ntitle: {ru: Товар склада}\ntypes: [{kind: external-data-source-table, reference: "+goodsTable+"}]\n")
	catalogID := uuid.MustNew().String()
	writeMetadata(t, root, CatalogKind, catalogID, "format: 1\nid: "+catalogID+"\nname: Контрагенты\ntitle: {ru: Контрагенты}\ncode: {type: string, length: 9, auto: true}\ndescription_length: 150\n"+
		"attributes:\n  - {id: "+uuid.MustNew().String()+", name: Товар, title: {ru: Товар}, types: [{kind: defined-type, reference: "+defined+"}]}\n")
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a defined type holding a reference to a source was refused: %v", err)
	}
	if _, err := catalog.ApplicationSchema(); err == nil || !strings.Contains(err.Error(), "is not a type of anything the infobase stores") {
		t.Fatalf("a stored field typed through a defined type by a reference to a source: %v", err)
	}
}
