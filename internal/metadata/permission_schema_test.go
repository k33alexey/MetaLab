package metadata

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestPermissionSchemaShowsEveryPartWhereTheTreeShowsIt catches the role
// editor's schema losing a part, putting it in the wrong folder or under the
// wrong parent, offering it the wrong rights, or showing an object that lies
// inside another without its owner: a column of a table part shown as an
// attribute of the document, a dimension of a register as a resource, a
// method of an HTTP service outside its template, a recalculation beside the
// registers instead of under its own. The expectations are written out here
// from the exports (Rights.xml of every role), not read from partGroups.
func TestPermissionSchemaShowsEveryPartWhereTheTreeShowsIt(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	const (
		operation = "f3000000-0000-4000-8000-0000000000a1"
		template  = "f4000000-0000-4000-8000-0000000000a1"
		method    = "f4000000-0000-4000-8000-0000000000a2"
	)
	writeMetadata(t, root, WebServiceKind, cmpWebService, "format: 1\nid: "+cmpWebService+"\nname: Обмен\ntitle: {ru: Обмен}\n"+
		"namespace: http://example.org/exchange/1.0\npackages: [{package: "+cmpXDTOPackage+"}]\n"+
		"operations:\n  - {id: "+operation+", name: Проверка, title: {ru: Проверка}, procedure: Проверка, "+
		"return_type: {namespace: \"http://www.w3.org/2001/XMLSchema\", name: string}}\n")
	writeModuleFile(t, filepath.Join(root, "metadata", string(WebServiceKind), "Обмен", "МодульСервиса.bsl"),
		"Функция Проверка()\n\tВозврат \"\";\nКонецФункции\n")
	writeHTTPService(t, root, cmpHTTPService, "Биллинг", "root_url: billing\ntemplates:\n  - {id: "+template+
		", name: Версия, title: {ru: Версия}, template: /version, methods: [{id: "+method+
		", name: Получить, title: {ru: Получить}, method: GET, handler: ВерсияПолучить}]}\n")
	writeModuleFile(t, filepath.Join(root, "metadata", string(HTTPServiceKind), "Биллинг", "МодульСервиса.bsl"),
		"Функция ВерсияПолучить(Запрос)\n\tВозврат Неопределено;\nКонецФункции\n")

	schema, err := LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	fieldRights, view, use := []PermissionOperation{PermissionView, PermissionEdit}, []PermissionOperation{PermissionView}, []PermissionOperation{PermissionUse}
	objects := map[string]PermissionObject{}
	for _, object := range schema.Objects {
		objects[object.ID.String()] = object
		for _, field := range object.Fields {
			if field.Operations == nil {
				t.Errorf("%s.%s offers null rights, which the editor cannot read", object.Name, field.Name)
			}
			if !slices.ContainsFunc(schema.PartGroups, func(group PermissionPartGroup) bool { return group.Key == field.Group }) {
				t.Errorf("%s.%s is in folder %q, which the schema does not name", object.Name, field.Name, field.Group)
			}
		}
	}
	type want struct {
		object, key, group, parent string
		rights                     []PermissionOperation
	}
	for _, expected := range []want{
		{cmpDocument, "number", "standard-attributes", "", fieldRights},
		{cmpDocument, cmpDocumentWarehouse, "attributes", "", fieldRights},
		{cmpPrices, cmpPricesGoods, "dimensions", "", fieldRights},
		{cmpPrices, cmpPricesPrice, "resources", "", fieldRights},
		{cmpAccounts, "extdimensiontypes", "standard-table-parts", "", fieldRights},
		{cmpAccounts, "extdimensiontypes.turnoversonly", "standard-attributes", "extdimensiontypes", fieldRights},
		{cmpWebService, operation, "operations", "", use},
		{cmpHTTPService, template, "url-templates", "", []PermissionOperation{}},
		{cmpHTTPService, method, "methods", template, use},
	} {
		object, ok := objects[expected.object]
		if !ok {
			t.Errorf("%s is not in the schema", expected.object)
			continue
		}
		index := slices.IndexFunc(object.Fields, func(field PermissionField) bool { return field.Key == expected.key })
		if index < 0 {
			t.Errorf("%s has no part %s", object.Name, expected.key)
			continue
		}
		field := object.Fields[index]
		if string(field.Group) != expected.group || field.Parent != expected.parent || !slices.Equal(field.Operations, expected.rights) {
			t.Errorf("%s.%s is %s under %q with %v, want %s under %q with %v", object.Name, field.Name,
				field.Group, field.Parent, field.Operations, expected.group, expected.parent, expected.rights)
		}
	}
	// A command is shown, with View alone.
	for _, object := range schema.Objects {
		for _, field := range object.Fields {
			if field.Group == CommandsPart && !slices.Equal(field.Operations, view) {
				t.Errorf("command %s.%s offers %v", object.Name, field.Name, field.Operations)
			}
		}
	}
	// What lies inside another names it; what lies in the configuration
	// itself names nothing.
	owners := map[string]string{cmpRecalculation: cmpPayroll, goodsTable: warehouseSource, salesCube: warehouseSource,
		goodsDimTable: salesCube}
	for id, owner := range owners {
		object, ok := objects[id]
		if !ok {
			t.Errorf("%s is not in the schema", id)
			continue
		}
		if object.Owner == nil || object.Owner.String() != owner {
			t.Errorf("%s %s lies in %v, want %s", object.Kind, object.Name, object.Owner, owner)
		}
	}
	if recalculation := objects[cmpRecalculation]; recalculation.Name != "Перерасчет" || recalculation.Title["ru"] != "Перерасчёт" {
		t.Errorf("the recalculation is shown as %q %v, want its own name and synonym", recalculation.Name, recalculation.Title)
	}
	for _, id := range []string{cmpDocument, cmpPayroll, warehouseSource, cmpSubsystem} {
		if object := objects[id]; object.Owner != nil {
			t.Errorf("%s lies in %s, and it lies in the configuration", object.Name, object.Owner)
		}
	}
	if len(schema.PartGroups) != len(partGroups) {
		t.Errorf("the schema names %d folders of parts, the model %d", len(schema.PartGroups), len(partGroups))
	}
}

func writeModuleFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The editor offers a restriction of access to data only on the rights the
// help names (read, insert, update, delete), and only on what keeps data.
// The expectations are written out here from the help («Ограничение доступа
// к данным»), not read from restrictedRights. Defect caught: the editor
// offering a restriction the server refuses at saving - on Просмотр, on a
// web service, on the configuration - or none on a document.
func TestPermissionSchemaOffersRestrictionsWhereDataIsKept(t *testing.T) {
	t.Parallel()
	schema, err := LoadPermissionSchema(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	all := []PermissionOperation{PermissionRead, PermissionInsert, PermissionUpdate, PermissionDelete}
	seen := map[string]bool{}
	for _, object := range schema.Objects {
		if object.RestrictedRights == nil {
			t.Errorf("%s offers null restricted rights, which the editor cannot read", object.Name)
		}
		for _, right := range object.RestrictedRights {
			if !slices.Contains(all, right) || !slices.Contains(object.Rights, right) {
				t.Errorf("%s offers a restriction on %q", object.Name, right)
			}
		}
		switch object.ID.String() {
		case cmpDocument:
			seen["document"] = true
			if !slices.Equal(object.RestrictedRights, all) {
				t.Errorf("document restricts %v, want %v", object.RestrictedRights, all)
			}
		case cmpWebService:
			seen["web service"] = true
			if len(object.RestrictedRights) != 0 {
				t.Errorf("web service restricts %v", object.RestrictedRights)
			}
		}
		if object.Root && len(object.RestrictedRights) != 0 {
			t.Errorf("the configuration restricts %v", object.RestrictedRights)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("objects checked: %v", seen)
	}
}
