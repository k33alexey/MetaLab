package studio

import (
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
	_, refused := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, Subsystems: &only}, opened.Revision)
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

	saved, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, Subsystems: &only}, opened.Revision)
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
	window := CatalogWindowValue{CatalogDefinition: opened.Catalog, Subsystems: &subsystems, FunctionalOptions: &options}
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
	if _, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, ExchangePlans: &wrong}, opened.Revision); err == nil {
		t.Error("an autoregistration other than allow and deny was written")
	}
	if text := read("Полный"); !strings.Contains(text[strings.Index(text, member):], "auto_record: deny") {
		t.Errorf("the refusal left the autoregistration of Товары in Полный changed:\n%s", text)
	}
	if read("Филиалы") != branchesBefore {
		t.Errorf("the refusal left Филиалы changed:\n%s", read("Филиалы"))
	}

	marks := []MemberPlace{{ID: full, AutoRecord: "allow"}, {ID: branches, AutoRecord: "deny"}}
	saved, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: opened.Catalog, ExchangePlans: &marks}, opened.Revision)
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
	if _, err := workspace.SaveCatalogWindow(goods, CatalogWindowValue{CatalogDefinition: saved.Catalog, ExchangePlans: &none}, saved.Revision); err != nil {
		t.Fatal(err)
	}
	if text := read("Полный"); strings.Contains(text, member) || !strings.Contains(text, partner) {
		t.Errorf("unmarked, Полный does not lose Товары alone:\n%s", text)
	}
	if _, err := metadata.Load(root); err != nil {
		t.Errorf("the plans written do not load: %v", err)
	}
}
