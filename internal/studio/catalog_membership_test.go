package studio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A save of the window of a catalog writes the subsystems its page marks -
// a new member last, an unmarked one taken out - and a catalog refused after
// them puts every subsystem written back as it was - its content, in the
// canonical form every write of Studio gives a description. Defects
// caught: a mark not written into the subsystem, a member left behind, a
// refusal of the catalog leaving the subsystems changed for a catalog that
// was not saved.
func TestTheWindowOfACatalogWritesItsSubsystems(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	const goods = "metadata/catalogs/Товары/object.yaml"
	sales := filepath.Join(root, "metadata", "subsystems", treeSalesSubsystem+".yaml")
	warehouse := filepath.Join(root, "metadata", "subsystems", treeWarehouseSubsystem+".yaml")
	read := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	opened, err := workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	var marked []string
	for _, place := range opened.Subsystems {
		if place.Member {
			marked = append(marked, place.Name)
		}
	}
	if !slices.Equal(marked, []string{"Склад"}) {
		t.Fatalf("Товары is shown in %v, want Склад", marked)
	}
	salesID, err := uuid.Parse(treeSalesSubsystem)
	if err != nil {
		t.Fatal(err)
	}
	only := []uuid.UUID{salesID}

	// Refused halfway: the folder of the catalog takes no file.
	folder := filepath.Join(root, "metadata", "catalogs", "Товары")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	_, refused := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, pageMarks: pageMarks{Subsystems: &only}}, opened.Revision)
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if refused == nil {
		t.Fatal("the catalog was written into a folder that takes no file")
	}
	const member = "10000000-0000-4000-8000-000000000101"
	if strings.Contains(read(sales), member) || !strings.Contains(read(warehouse), member) {
		t.Errorf("the refusal left the subsystems changed:\n%s\n%s", read(sales), read(warehouse))
	}

	saved, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, pageMarks: pageMarks{Subsystems: &only}}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if text := read(sales); !strings.HasSuffix(strings.TrimSpace(text), member) {
		t.Errorf("Товары is not the last member of Продажи:\n%s", text)
	}
	if text := read(warehouse); strings.Contains(text, member) {
		t.Errorf("Товары is left in Склад:\n%s", text)
	}
	for _, place := range saved.Subsystems {
		if place.Member != (place.Name == "Продажи") {
			t.Errorf("after the save %s is shown marked %v", place.Name, place.Member)
		}
	}
}

// The options of addCatalogOptions: Склады holds Товары itself, Артикулы
// only its attribute Артикул.
const (
	optionWarehouses = "30000000-0000-4000-8000-000000000401"
	optionArticles   = "30000000-0000-4000-8000-000000000402"
)

// addCatalogOptions gives the demo project two functional options kept in
// the attribute Артикул of Товары: Склады, whose content is Товары itself,
// and Артикулы, whose content is that attribute alone.
func addCatalogOptions(t *testing.T, root string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", "functional-options")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	const goods, article = "10000000-0000-4000-8000-000000000101", "10000000-0000-4000-8000-000000001101"
	location := "location: {kind: catalogs, object: " + goods + ", element: " + article + "}\n"
	for id, body := range map[string]string{
		optionWarehouses: "format: 1\nid: " + optionWarehouses + "\nname: Склады\ntitle: {ru: Склады}\n" + location +
			"content:\n  - {kind: catalogs, object: " + goods + "}\n",
		optionArticles: "format: 1\nid: " + optionArticles + "\nname: Артикулы\ntitle: {ru: Артикулы}\n" + location +
			"content:\n  - {kind: catalogs, object: " + goods + ", element: " + article + "}\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, id+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A save of the window of a catalog writes the options its page
// «Функциональные опции» marks: the catalog itself put last into the
// content of a marked option, beside the items naming its parts, and taken
// out of an unmarked one, its parts left there; a refusal of the options
// after the subsystems are written puts the subsystems back. Defects
// caught: a part of the catalog taken for the catalog - shown marked, or
// taken out with it; a mark not written into the option; an option refused
// leaving the subsystem written before it changed.
func TestTheWindowOfACatalogWritesItsOptions(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	addCatalogOptions(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	const goods = "metadata/catalogs/Товары/object.yaml"
	read := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	opened, err := workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	var shown []string
	for _, place := range opened.FunctionalOptions {
		shown = append(shown, place.Name+map[bool]string{true: " ✓"}[place.Member])
	}
	if !slices.Equal(shown, []string{"Артикулы", "Склады ✓"}) {
		t.Fatalf("the options are shown as %v, want Артикулы, Склады ✓", shown)
	}
	articles, err := uuid.Parse(optionArticles)
	if err != nil {
		t.Fatal(err)
	}
	sales, err := uuid.Parse(treeSalesSubsystem)
	if err != nil {
		t.Fatal(err)
	}
	subsystems, options := []uuid.UUID{sales}, []uuid.UUID{articles}
	window := CatalogWindowValue{CatalogDefinition: opened.Catalog, pageMarks: pageMarks{Subsystems: &subsystems, FunctionalOptions: &options}}
	salesFile := filepath.Join(root, "metadata", "subsystems", treeSalesSubsystem+".yaml")
	articlesFile := filepath.Join(root, "metadata", "functional-options", optionArticles+".yaml")
	warehousesFile := filepath.Join(root, "metadata", "functional-options", optionWarehouses+".yaml")
	before := map[string]string{salesFile: read(salesFile), articlesFile: read(articlesFile), warehousesFile: read(warehousesFile)}

	// Refused halfway: the subsystems are written, the folder of the options
	// takes no file.
	folder := filepath.Join(root, "metadata", "functional-options")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	_, refused := workspace.SaveCatalogWindow(goods, window, opened.Revision)
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if refused == nil {
		t.Fatal("the options were written into a folder that takes no file")
	}
	const member = "10000000-0000-4000-8000-000000000101"
	if strings.Contains(read(salesFile), member) {
		t.Errorf("the refusal of the options left the subsystem changed:\n%s", read(salesFile))
	}
	for path, text := range before {
		if path != salesFile && read(path) != text {
			t.Errorf("the refusal left %s changed:\n%s", filepath.Base(path), read(path))
		}
	}

	saved, err := workspace.SaveCatalogWindow(goods, window, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	text := read(articlesFile)
	text = text[strings.Index(text, "content:"):]
	part, whole := strings.Index(text, "element: 10000000-0000-4000-8000-000000001101"), strings.LastIndex(text, "object: "+member)
	if part < 0 || whole < part || strings.Count(text, "object: "+member) != 2 {
		t.Errorf("Товары is not put last into Артикулы beside its attribute:\n%s", text)
	}
	if text := read(warehousesFile); strings.Contains(text, "content") {
		t.Errorf("Товары is left in Склады:\n%s", text)
	}
	for _, place := range saved.FunctionalOptions {
		if place.Member != (place.Name == "Артикулы") {
			t.Errorf("after the save %s is shown marked %v", place.Name, place.Member)
		}
	}
	if _, err := metadata.Load(root); err != nil {
		t.Errorf("the options written do not load: %v", err)
	}
}

// The exchange plans of addCatalogExchangePlans: Полный registers Товары by
// the application's code, Филиалы registers nothing.
const (
	planFull     = "30000000-0000-4000-8000-000000000501"
	planBranches = "30000000-0000-4000-8000-000000000502"
)

// addCatalogExchangePlans gives the demo project two exchange plans: Полный,
// whose content lists Товары with autoregistration denied and Контрагенты
// allowed, and Филиалы, whose content is empty.
func addCatalogExchangePlans(t *testing.T, root string) {
	t.Helper()
	const goods, partners = "10000000-0000-4000-8000-000000000101", "10000000-0000-4000-8000-000000000102"
	for name, body := range map[string]string{
		"Полный": "format: 1\nid: " + planFull + "\nname: Полный\ntitle: {ru: Полный}\ncode: {type: string, length: 9}\ndescription_length: 25\n" +
			"content:\n  - {kind: catalogs, object: " + goods + ", auto_record: deny}\n  - {kind: catalogs, object: " + partners + ", auto_record: allow}\n",
		"Филиалы": "format: 1\nid: " + planBranches + "\nname: Филиалы\ntitle: {ru: Филиалы}\ncode: {type: string, length: 9}\ndescription_length: 25\n",
	} {
		directory := filepath.Join(root, "metadata", "exchange-plans", name)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "object.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A save of the window of a catalog writes the exchange plans its page
// «Обмен данными» marks: a plan marked anew lists the catalog last with the
// autoregistration chosen, a plan whose autoregistration changes keeps the
// catalog where it stood, and a plan unmarked takes it out, the other
// objects of its content left; an autoregistration other than the two is
// refused by the write of the plan, and the plan written before it put
// back. Defects caught: a mark not written, the
// autoregistration lost or the catalog moved, another object of the plan
// taken out with it, a plan written with an autoregistration the model
// does not know.
func TestTheWindowOfACatalogWritesItsExchangePlans(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	addCatalogExchangePlans(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	const goods = "metadata/catalogs/Товары/object.yaml"
	read := func(name string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(root, "metadata", "exchange-plans", name, "object.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	opened, err := workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	var shown []string
	for _, place := range opened.ExchangePlans {
		shown = append(shown, place.Name+map[bool]string{true: " ✓ " + string(place.AutoRecord)}[place.Member])
	}
	if !slices.Equal(shown, []string{"Полный ✓ deny", "Филиалы"}) {
		t.Fatalf("the exchange plans are shown as %v, want Полный ✓ deny, Филиалы", shown)
	}
	full, err := uuid.Parse(planFull)
	if err != nil {
		t.Fatal(err)
	}
	branches, err := uuid.Parse(planBranches)
	if err != nil {
		t.Fatal(err)
	}
	const member, partner = "object: 10000000-0000-4000-8000-000000000101", "object: 10000000-0000-4000-8000-000000000102"
	branchesBefore := read("Филиалы")

	// Полный is written first and put back - in the canonical form every
	// write of Studio gives a description - when Филиалы is refused.
	wrong := []MemberPlace{{ID: full, AutoRecord: "allow"}, {ID: branches, AutoRecord: "sometimes"}}
	if _, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, pageMarks: pageMarks{ExchangePlans: &wrong}}, opened.Revision); err == nil {
		t.Error("an autoregistration other than allow and deny was written")
	}
	if text := read("Полный"); !strings.Contains(text[strings.Index(text, member):], "auto_record: deny") {
		t.Errorf("the refusal left the autoregistration of Товары in Полный changed:\n%s", text)
	}
	if read("Филиалы") != branchesBefore {
		t.Errorf("the refusal left Филиалы changed:\n%s", read("Филиалы"))
	}

	marks := []MemberPlace{{ID: full, AutoRecord: "allow"}, {ID: branches, AutoRecord: "deny"}}
	saved, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, pageMarks: pageMarks{ExchangePlans: &marks}}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	text := read("Полный")
	if at, next := strings.Index(text, member), strings.Index(text, partner); at < 0 || at > next || !strings.Contains(text[at:next], "auto_record: allow") {
		t.Errorf("Товары is not kept first in Полный with autoregistration allowed:\n%s", text)
	}
	if text := read("Филиалы"); !strings.Contains(text, member) || !strings.Contains(text, "auto_record: deny") {
		t.Errorf("Товары is not put into Филиалы with autoregistration denied:\n%s", text)
	}
	for _, place := range saved.ExchangePlans {
		if !place.Member || place.AutoRecord != map[string]metadata.AutoRecord{"Полный": "allow", "Филиалы": "deny"}[place.Name] {
			t.Errorf("after the save %s is shown %v %q", place.Name, place.Member, place.AutoRecord)
		}
	}

	none := []MemberPlace{}
	if _, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: saved.Catalog, pageMarks: pageMarks{ExchangePlans: &none}}, saved.Revision); err != nil {
		t.Fatal(err)
	}
	if text := read("Полный"); strings.Contains(text, member) || !strings.Contains(text, partner) {
		t.Errorf("unmarked, Полный does not lose Товары alone:\n%s", text)
	}
	if _, err := metadata.Load(root); err != nil {
		t.Errorf("the plans written do not load: %v", err)
	}
}

// enterSalesOnGoods makes the document ПродажаТоваров of the demo project
// entered on the basis of the catalog Товары, and so a table of object data
// Склад.Остатки of an external source it gives the project.
func enterSalesOnGoods(t *testing.T, root string) string {
	t.Helper()
	source := filepath.Join(root, "metadata", "external-data-sources", "Склад")
	for path, body := range map[string]string{
		filepath.Join(source, "object.yaml"): "format: 1\nid: 30000000-0000-4000-8000-000000000601\nname: Склад\ntitle: {ru: Склад}\n",
		filepath.Join(source, "tables", "Остатки", "object.yaml"): "format: 1\nid: 30000000-0000-4000-8000-000000000602\nname: Остатки\ntitle: {ru: Остатки}\n" +
			"name_in_data_source: dbo.Stock\ndata_type: object\nkey_fields: [Код]\nfields:\n  - id: 30000000-0000-4000-8000-000000000603\n" +
			"    name: Код\n    title: {ru: Код}\n    types: [{kind: string, length: 20}]\n    name_in_data_source: code\n" +
			"based_on:\n  - 10000000-0000-4000-8000-000000000101\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sales := filepath.Join(root, "metadata", "documents", "ПродажаТоваров", "object.yaml")
	content, err := os.ReadFile(sales)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sales, append(content, []byte("based_on:\n  - 10000000-0000-4000-8000-000000000101\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	return sales
}

// A save of the window of a catalog writes the list «Является основанием
// для» of its page «Ввод на основании» into the lists of bases of the
// objects named - a table of an external source as much as a catalog or a
// document - the catalog put last into a list naming it anew, taken out
// of one that named it - and a refusal of the catalog puts them back; the
// catalog itself is not among them, its own list is the window's. Defects
// caught: the list read off the wrong side; a mark not written into the
// other object; a refusal leaving the other objects changed; the catalog
// written twice by one save when it is its own basis.
func TestTheWindowOfACatalogWritesWhatItIsTheBasisFor(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	sales := enterSalesOnGoods(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	const goods, partnersFile = "metadata/catalogs/Товары/object.yaml", "metadata/catalogs/Контрагенты/object.yaml"
	read := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	opened, err := workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	var shown []string
	for _, place := range opened.BasisFor {
		shown = append(shown, place.Name)
	}
	if !slices.Equal(shown, []string{"Документ.ПродажаТоваров", "ВнешнийИсточникДанных.Склад.Таблица.Остатки"}) {
		t.Fatalf("Товары is shown the basis for %v, want Документ.ПродажаТоваров and the table Склад.Остатки", shown)
	}
	stock := filepath.Join(root, "metadata", "external-data-sources", "Склад", "tables", "Остатки", "object.yaml")
	partners, err := uuid.Parse("10000000-0000-4000-8000-000000000102")
	if err != nil {
		t.Fatal(err)
	}
	marked := []uuid.UUID{partners, opened.Catalog.ID}
	value := opened.Catalog
	value.BasedOn = []uuid.UUID{opened.Catalog.ID}
	window := CatalogWindowValue{CatalogDefinition: value, pageMarks: pageMarks{BasisFor: &marked}}
	const member = "10000000-0000-4000-8000-000000000101"
	salesBefore := read(sales)

	// Refused halfway: the folder of the catalog takes no file.
	folder := filepath.Join(root, "metadata", "catalogs", "Товары")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	_, refused := workspace.SaveCatalogWindow(goods, window, opened.Revision)
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if refused == nil {
		t.Fatal("the catalog was written into a folder that takes no file")
	}
	if text := read(filepath.Join(root, partnersFile)); strings.Contains(text, "based_on") {
		t.Errorf("the refusal left Контрагенты entered on the basis of Товары:\n%s", text)
	}
	if text := read(sales); !strings.Contains(text, "based_on") {
		t.Errorf("the refusal left ПродажаТоваров without its basis:\n%s (before:\n%s)", text, salesBefore)
	}

	saved, err := workspace.SaveCatalogWindow(goods, window, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if text := read(filepath.Join(root, partnersFile)); !strings.Contains(text, "based_on:\n  - "+member) {
		t.Errorf("Контрагенты is not entered on the basis of Товары:\n%s", text)
	}
	if text := read(sales); strings.Contains(text, "based_on") {
		t.Errorf("ПродажаТоваров is left entered on the basis of Товары:\n%s", text)
	}
	if text := read(stock); strings.Contains(text, "based_on") {
		t.Errorf("the table Склад.Остатки is left entered on the basis of Товары:\n%s", text)
	}
	if text := read(filepath.Join(root, goods)); !strings.Contains(text, "based_on:\n  - "+member) {
		t.Errorf("Товары has not its own list:\n%s", text)
	}
	shown = nil
	for _, place := range saved.BasisFor {
		shown = append(shown, place.Name)
	}
	if !slices.Equal(shown, []string{"Справочник.Контрагенты"}) {
		t.Errorf("after the save Товары is shown the basis for %v", shown)
	}
	if _, err := metadata.Load(root); err != nil {
		t.Errorf("the lists written do not load: %v", err)
	}
}

// mustID is the identifier written as text.
func mustID(t *testing.T, text string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// addCatalogRoles gives the demo project two roles: Кладовщик, reading
// Товары under a restriction and Контрагенты too, and Менеджер, reading
// Контрагенты alone.
func addCatalogRoles(t *testing.T, workspace *Workspace) {
	t.Helper()
	goods, partners := mustID(t, "10000000-0000-4000-8000-000000000101"), mustID(t, "10000000-0000-4000-8000-000000000102")
	for _, name := range []string{"Кладовщик", "Менеджер"} {
		role, err := workspace.CreateRole(name)
		if err != nil {
			t.Fatal(err)
		}
		read := []metadata.PermissionOperation{metadata.PermissionRead}
		role.Role.Objects = []metadata.ObjectPermission{{Object: partners, Operations: read}}
		if name == "Кладовщик" {
			role.Role.Objects = append([]metadata.ObjectPermission{
				{Object: goods, Operations: read, Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionRead, Condition: "Истина"}}},
			}, role.Role.Objects...)
		}
		if _, err := workspace.SaveRole(role.Path, role.Role, role.Revision); err != nil {
			t.Fatal(err)
		}
	}
}

// A save of the window of a catalog writes what its page «Права» changes in
// the roles - the catalog's entry alone, another object of the role left as
// it was - and refuses, writing nothing, a change of a role changed on disk
// since the window read it and one the role may not say; a catalog refused
// after the roles puts them back. Defects caught: the page shown the whole
// role, or a save taking the role's other objects away; a role changed in
// another window overwritten; a right on a part the catalog has not, or a
// restriction the role may not keep, written;
// a role refused halfway left changed.
func TestTheWindowOfACatalogWritesItsRights(t *testing.T) {
	root := copyDemoProjectForStudioTest(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	addCatalogRoles(t, workspace)
	const goods = "metadata/catalogs/Товары/object.yaml"
	opened, err := workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	var shown []string
	for _, place := range opened.Roles {
		shown = append(shown, fmt.Sprintf("%s %d", place.Role.Name, len(place.Role.Objects)))
	}
	if !slices.Equal(shown, []string{"Кладовщик 1", "Менеджер 0"}) {
		t.Fatalf("the roles are shown as %v, want Кладовщик with one object, Менеджер with none", shown)
	}
	full, err := metadata.LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	own := full.Objects[slices.IndexFunc(full.Objects, func(item metadata.PermissionObject) bool { return item.ID == opened.Catalog.ID })]
	if schema := opened.RightsSchema; len(schema.Objects) != 1 || !slices.Equal(schema.Objects[0].Rights, own.Rights) || len(schema.Requires) == 0 {
		t.Fatalf("the schema of the page is %+v, want the catalog alone with its rights %v and what they require", schema.Objects, own.Rights)
	}
	keeper, manager := opened.Roles[0], opened.Roles[1]
	view := []metadata.PermissionOperation{metadata.PermissionRead, metadata.PermissionView}
	marks := []RoleMark{{ID: keeper.Role.ID, Revision: keeper.Revision},
		{ID: manager.Role.ID, Revision: manager.Revision, Permission: &metadata.ObjectPermission{Object: opened.Catalog.ID, Operations: view}}}
	window := CatalogWindowValue{CatalogDefinition: opened.Catalog, pageMarks: pageMarks{Roles: &marks}}
	roleOf := func(name string) metadata.RoleDefinition {
		t.Helper()
		loaded, err := metadata.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		role, _ := loaded.Role(name)
		return role
	}
	says := func(role metadata.RoleDefinition, object uuid.UUID) string {
		if entry := role.PermissionOf(object); entry != nil {
			return fmt.Sprint(entry.Operations, len(entry.Restrictions))
		}
		return "nothing"
	}

	// A right on a part the catalog has not, and a restriction of a right
	// the role neither grants nor denies, are refused before anything is
	// written.
	for what, entry := range map[string]metadata.ObjectPermission{
		"a right on a part the catalog has not": {Object: opened.Catalog.ID, Operations: view,
			Fields: []metadata.FieldPermission{{Field: "10000000-0000-4000-8000-00000000ffff", Operations: []metadata.PermissionOperation{metadata.PermissionView}}}},
		"a restriction of a right the role does not write": {Object: opened.Catalog.ID, Operations: view,
			Restrictions: []metadata.RightRestriction{{Right: metadata.PermissionUpdate, Condition: "Истина"}}},
	} {
		wrong := []RoleMark{{ID: manager.Role.ID, Revision: manager.Revision, Permission: &entry}}
		if _, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, pageMarks: pageMarks{Roles: &wrong}}, opened.Revision); err == nil {
			t.Errorf("%s was written", what)
		}
		if got := says(roleOf("Менеджер"), opened.Catalog.ID); got != "nothing" {
			t.Errorf("%s refused left Менеджер saying %s", what, got)
		}
	}

	// Refused halfway: the folder of the catalog takes no file.
	folder := filepath.Join(root, "metadata", "catalogs", "Товары")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatal(err)
	}
	_, refused := workspace.SaveCatalogWindow(goods, window, opened.Revision)
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if refused == nil {
		t.Fatal("the catalog was written into a folder that takes no file")
	}
	if got := says(roleOf("Кладовщик"), opened.Catalog.ID); got != "[read] 1" {
		t.Errorf("the refusal left Кладовщик saying %s of Товары", got)
	}
	if got := says(roleOf("Менеджер"), opened.Catalog.ID); got != "nothing" {
		t.Errorf("the refusal left Менеджер saying %s of Товары", got)
	}

	// Кладовщик changed in another window: its change is refused, and
	// nothing is written.
	other, err := workspace.ReadRole(keeper.Path)
	if err != nil {
		t.Fatal(err)
	}
	other.Role.Comment = "изменена в окне роли"
	changed, err := workspace.SaveRole(other.Path, other.Role, other.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SaveCatalogWindow(goods, window, opened.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Errorf("a role changed on disk was overwritten: %v", err)
	}
	if got := says(roleOf("Менеджер"), opened.Catalog.ID); got != "nothing" {
		t.Errorf("the refusal of Кладовщик left Менеджер saying %s", got)
	}

	// The window read anew: Кладовщик loses Товары and keeps Контрагенты
	// and the comment written in its own window; Менеджер gets its rights.
	opened, err = workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	marks[0].Revision, marks[1].Revision = opened.Roles[0].Revision, opened.Roles[1].Revision
	saved, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, pageMarks: pageMarks{Roles: &marks}}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	keeperRole := roleOf("Кладовщик")
	if got := says(keeperRole, opened.Catalog.ID); got != "nothing" {
		t.Errorf("Кладовщик says %s of Товары after the save", got)
	}
	if got := says(keeperRole, mustID(t, "10000000-0000-4000-8000-000000000102")); got != "[read] 0" || keeperRole.Comment != changed.Role.Comment {
		t.Errorf("Кладовщик lost what else it says: %s, %q", got, keeperRole.Comment)
	}
	if got := says(roleOf("Менеджер"), opened.Catalog.ID); got != "[read view] 0" {
		t.Errorf("Менеджер says %s of Товары after the save", got)
	}
	if saved.Roles[0].Revision == opened.Roles[0].Revision || len(saved.Roles[1].Role.Objects) != 1 {
		t.Errorf("the answer of the save shows the roles as read before it: %+v", saved.Roles)
	}

	// Менеджер changed again: its entry is replaced where it stands, its
	// other object kept.
	insert := []metadata.PermissionOperation{metadata.PermissionRead, metadata.PermissionInsert}
	again := []RoleMark{{ID: manager.Role.ID, Revision: saved.Roles[1].Revision, Permission: &metadata.ObjectPermission{Object: opened.Catalog.ID, Operations: insert}}}
	if _, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: saved.Catalog, pageMarks: pageMarks{Roles: &again}}, saved.Revision); err != nil {
		t.Fatal(err)
	}
	managerRole := roleOf("Менеджер")
	if got := says(managerRole, opened.Catalog.ID); got != "[read insert] 0" {
		t.Errorf("Менеджер says %s of Товары after the second save", got)
	}
	if got := says(managerRole, mustID(t, "10000000-0000-4000-8000-000000000102")); got != "[read] 0" {
		t.Errorf("Менеджер lost Контрагенты: %s", got)
	}
}
