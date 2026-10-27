package studio

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The prototype's words, written out here and not read from nameRefusals:
// docs/block-00/obshchee.md, «Имя и синоним».
const (
	refusedEmpty     = "Имя не должно быть пустым!"
	refusedRule      = `Неверное имя! Имя должно состоять из одного слова, начинаться с буквы и не содержать специальных символов кроме "_".`
	refusedNotUnique = "Имя не уникально!"
)

// TestTheNameOfAnObjectIsCheckedInItsBranch catches the name of an object
// weighed outside its branch or by words of its own: the rule, then the
// objects of its kind only - a role may share a catalog's name - and its own
// description left out, so that a change of case alone is no repeat (the
// former check of a catalog read its own folder as another catalog). A
// creation and a save are refused with the same words.
func TestTheNameOfAnObjectIsCheckedInItsBranch(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	const goods = "metadata/catalogs/Товары/object.yaml"
	for _, test := range []struct {
		path, name, want string
	}{
		{goods, "ТОВАРЫ", ""},
		{goods, "контрагенты", refusedNotUnique},
		{goods, "Номенклатура", ""},
		{goods, "", refusedEmpty},
		{goods, "1Товары", refusedRule},
		{goods, "Товары-2", refusedRule},
		{goods, "_Товары", ""},
		// The branch of documents holds the branches of numerators and of
		// sequences, which are no documents.
		{"metadata/documents/ПродажаТоваров/object.yaml", "Нумераторы", ""},
		{"metadata/documents/ПродажаТоваров/object.yaml", "поступлениетоваров", refusedNotUnique},
		{"configuration.yaml", "Продажи и склад", refusedRule},
		{"configuration.yaml", "ПродажиИСклад", ""},
	} {
		answer, err := workspace.CheckName(NameCheck{Path: test.path, Name: test.name})
		if err != nil || answer.Refusal != test.want {
			t.Errorf("%s named %q: %q, %v; want %q", test.path, test.name, answer.Refusal, err, test.want)
		}
	}
	for name, want := range map[string]string{"контрагенты": refusedNotUnique, "Цены прайса": refusedRule} {
		if _, err := workspace.CreateCatalog(name); err == nil || err.Error() != want {
			t.Errorf("a catalog created as %q: %v, want %q", name, err, want)
		}
	}
	role, err := workspace.CreateRole("Товары")
	if err != nil {
		t.Fatalf("a role is refused the name of a catalog: %v", err)
	}
	if _, err := workspace.CreateRole("товары"); err == nil || err.Error() != refusedNotUnique {
		t.Errorf("a second role «товары»: %v, want %q", err, refusedNotUnique)
	}
	opened, err := workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	renamed := opened.Catalog
	renamed.Name = "Контрагенты"
	if _, err := workspace.SaveCatalogEditor(goods, renamed, opened.Revision); err == nil || err.Error() != refusedNotUnique {
		t.Errorf("a catalog saved under another's name: %v, want %q", err, refusedNotUnique)
	}
	if answer, err := workspace.CheckName(NameCheck{Path: role.Path, Name: "ТОВАРЫ"}); err != nil || answer.Refusal != "" {
		t.Errorf("a role renamed by case alone: %q, %v", answer.Refusal, err)
	}
}

// TestTheNameOfAPartIsCheckedAsTheReadingChecksIt catches the name of an
// attribute or a table part weighed otherwise than the reading of the
// description weighs it: by its own branch, the names the platform keeps for
// the kind, and the rule that a table part is not named as an attribute
// (METADATA-OBJECTS.md, «Где имя уникально»). Two defects in particular: the
// first of two attributes alike was let through, since the reading refuses
// the later one; and an attribute named as a table part was let through,
// since the reading refuses the table part. And a name that fits is no repeat
// because two others repeat each other.
func TestTheNameOfAPartIsCheckedAsTheReadingChecksIt(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	const goods = "metadata/catalogs/Товары/object.yaml"
	opened, err := workspace.ReadCatalogEditor(goods)
	if err != nil {
		t.Fatal(err)
	}
	text := func(name string) metadata.LocalizedText { return metadata.LocalizedText{"ru": name} }
	field := func(name string) metadata.Attribute {
		return metadata.Attribute{ID: uuid.MustNew(), Name: name, Title: text(name), Types: []metadata.Type{{Kind: metadata.StringType, Length: 10}}}
	}
	base := opened.Catalog
	base.Attributes = append(base.Attributes, field("Цена"))
	base.TableParts = []metadata.TablePart{{ID: uuid.MustNew(), Name: "Партии", Title: text("Партии"), Attributes: []metadata.Attribute{field("Количество")}}}
	if _, err := metadata.DecodeCatalog(goods, encodeForTest(t, base), projectOf(t, workspace)); err != nil {
		t.Fatalf("the catalog of the test does not read: %v", err)
	}
	for _, test := range []struct {
		what string
		edit func(catalog *metadata.CatalogDefinition)
		at   []string
		want string
	}{
		{"an attribute as it is", func(*metadata.CatalogDefinition) {}, []string{"attributes", "0"}, ""},
		{"the first of two attributes alike", func(c *metadata.CatalogDefinition) { c.Attributes[0].Name = "цена" }, []string{"attributes", "0"}, refusedNotUnique},
		{"the second of two attributes alike", func(c *metadata.CatalogDefinition) { c.Attributes[1].Name = "АРТИКУЛ" }, []string{"attributes", "1"}, refusedNotUnique},
		{"an attribute beside two alike", func(c *metadata.CatalogDefinition) { c.Attributes = append(c.Attributes, field("цена")) },
			[]string{"attributes", "0"}, ""},
		{"an attribute named as a standard one", func(c *metadata.CatalogDefinition) { c.Attributes[1].Name = "Код" }, []string{"attributes", "1"}, refusedNotUnique},
		{"an attribute named as a table part", func(c *metadata.CatalogDefinition) { c.Attributes[1].Name = "Партии" }, []string{"attributes", "1"}, refusedNotUnique},
		{"a table part named as an attribute", func(c *metadata.CatalogDefinition) { c.TableParts[0].Name = "Цена" }, []string{"tableParts", "0"}, refusedNotUnique},
		{"an attribute of a table part named as one of the catalog", func(c *metadata.CatalogDefinition) { c.TableParts[0].Attributes[0].Name = "Цена" },
			[]string{"tableParts", "0", "attributes", "0"}, ""},
		{"an empty name", func(c *metadata.CatalogDefinition) { c.Attributes[0].Name = "" }, []string{"attributes", "0"}, refusedEmpty},
		{"a name that breaks the rule", func(c *metadata.CatalogDefinition) { c.TableParts[0].Attributes[0].Name = "2шт" },
			[]string{"tableParts", "0", "attributes", "0"}, refusedRule},
	} {
		catalog := cloneForTest(t, base)
		test.edit(&catalog)
		description, err := json.Marshal(catalog)
		if err != nil {
			t.Fatal(err)
		}
		answer, err := workspace.CheckName(NameCheck{Path: goods, Description: description, At: test.at})
		if err != nil || answer.Refusal != test.want {
			t.Errorf("%s: %q, %v; want %q", test.what, answer.Refusal, err, test.want)
		}
	}
}

// TestTheNameOfALanguageIsCheckedAmongTheLanguages catches a language weighed
// against what is saved and not against the list its editor holds: the
// names of the other languages, unsaved too, in any case.
func TestTheNameOfALanguageIsCheckedAmongTheLanguages(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	configuration := projectOf(t, workspace)
	for name, want := range map[string]string{"english": refusedNotUnique, "Русский": refusedNotUnique, "Українська": "", "Англійська": "", "": refusedEmpty} {
		edited := configuration
		edited.Languages = append(edited.Languages[:0:0], configuration.Languages...)
		edited.Languages[1].Name = name
		description, err := json.Marshal(edited)
		if err != nil {
			t.Fatal(err)
		}
		answer, err := workspace.CheckName(NameCheck{Path: "configuration.yaml", Description: description, At: []string{"languages", "1"}})
		if err != nil || answer.Refusal != want {
			t.Errorf("the second language named %q: %q, %v; want %q", name, answer.Refusal, err, want)
		}
	}
}

func projectOf(t *testing.T, workspace *Workspace) project.Project {
	t.Helper()
	configuration, err := project.ValidateLayout(workspace.root)
	if err != nil {
		t.Fatal(err)
	}
	return configuration
}

func encodeForTest(t *testing.T, value any) io.Reader {
	t.Helper()
	var encoded bytes.Buffer
	if err := metadata.Encode(&encoded, value); err != nil {
		t.Fatal(err)
	}
	return &encoded
}

// cloneForTest is a catalog of its own: an edit of a case leaves the next
// one's lists alone.
func cloneForTest(t *testing.T, catalog metadata.CatalogDefinition) metadata.CatalogDefinition {
	t.Helper()
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	var clone metadata.CatalogDefinition
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

// TestANewObjectHasNoSynonym catches a new catalog or role given its name as
// its synonym: the prototype's new object stands with an empty synonym
// (block-00, konstanta.md, snapshot novaya-osnovnye), and the name is shown
// in its place until a name typed makes one.
func TestANewObjectHasNoSynonym(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := workspace.CreateCatalog("Склады")
	if err != nil {
		t.Fatal(err)
	}
	role, err := workspace.CreateRole("Кладовщик")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Catalog.Title) != 0 || len(role.Role.Title) != 0 {
		t.Errorf("a new catalog has the synonym %v, a new role %v; want none", catalog.Catalog.Title, role.Role.Title)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Errorf("a project with objects of no synonym does not load: %v", err)
	}
}

// numberedInNode runs numberedName of studio-common.js on every case read
// from stdin and prints the answers in the same order.
const numberedInNode = `
const {readFileSync} = await import('node:fs');
const vm = await import('node:vm');
const context = vm.createContext({});
vm.runInContext(readFileSync(process.argv[1], 'utf8'), context);
context.cases = JSON.parse(readFileSync(0, 'utf8'));
process.stdout.write(JSON.stringify(vm.runInContext(
  'cases.map(item => numberedName(item.base, new Set(item.taken.map(name => name.toLocaleLowerCase()))))', context)));
`

// TestANewNameIsNumberedLikeOnThePage catches «Добавить» naming otherwise
// than the prototype - «Справочник1», «Реквизит1», the first number from 1
// that is free, in any case (docs/block-00/obshchee.md) - and the page,
// which names the parts, drifting from the server, which names the objects.
// The former page gave the base alone first and then «База2».
func TestANewNameIsNumberedLikeOnThePage(t *testing.T) {
	type numbered struct {
		Base  string   `json:"base"`
		Taken []string `json:"taken"`
		want  string
	}
	cases := []numbered{
		{"Реквизит", []string{}, "Реквизит1"},
		{"Реквизит", []string{"Реквизит"}, "Реквизит1"},
		{"Реквизит", []string{"РЕКВИЗИТ1"}, "Реквизит2"},
		{"Реквизит", []string{"Реквизит2"}, "Реквизит1"},
		{"Справочник", []string{"Справочник1", "справочник3"}, "Справочник2"},
	}
	for _, item := range cases {
		if got := numberedName(item.Base, item.Taken); got != item.want {
			t.Errorf("the server names %q beside %v %q, want %q", item.Base, item.Taken, got, item.want)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the page's half of this test")
	}
	payload, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	var answers []string
	if err := json.Unmarshal(runInNode(t, node, numberedInNode, payload, "ui/studio-common.js"), &answers); err != nil || len(answers) != len(cases) {
		t.Fatalf("node answered %v: %v", answers, err)
	}
	for index, item := range cases {
		if answers[index] != item.want {
			t.Errorf("the page names %q beside %v %q, want %q", item.Base, item.Taken, answers[index], item.want)
		}
	}
}

// TestAddMakesTheKindsNextObject catches «Добавить» asking a name or naming
// the object otherwise than by its kind's word and the next number among its
// branch: a catalog beside Справочник1 is Справочник2, a role is Роль1 - the
// roles are another branch.
func TestAddMakesTheKindsNextObject(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for range 2 {
		created, err := workspace.CreateCatalog("")
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, created.Catalog.Name)
	}
	role, err := workspace.CreateRole("")
	if err != nil {
		t.Fatal(err)
	}
	names = append(names, role.Role.Name)
	if want := []string{"Справочник1", "Справочник2", "Роль1"}; !slices.Equal(names, want) {
		t.Errorf("«Добавить» named %v, want %v", names, want)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Errorf("the project with the new objects does not load: %v", err)
	}
}

// TestARenamedCatalogTakesItsFolderAlong catches a catalog that cannot be
// renamed in its window: its folder is named after it, and a save under a
// new name was refused - «object Склады lies in a folder called
// Справочник1» - so that «Добавить», which names the catalog Справочник1,
// left a name that could not be put right. The folder goes with the name,
// its forms with it, and a save refused after the move puts it back.
func TestARenamedCatalogTakesItsFolderAlong(t *testing.T) {
	t.Parallel()
	root := copyDemoProjectForStudioTest(t)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.CreateCatalog("")
	if err != nil {
		t.Fatal(err)
	}
	refused := created.Catalog
	refused.Name, refused.Code.Length = "Склады", 51
	if _, err := workspace.SaveCatalogEditor(created.Path, refused, created.Revision); err == nil {
		t.Fatal("a catalog of code length 51 was saved")
	}
	if _, err := workspace.ReadCatalogEditor(created.Path); err != nil {
		t.Fatalf("a refused save left the catalog away from its folder: %v", err)
	}
	renamed := created.Catalog
	renamed.Name = "Склады"
	saved, err := workspace.SaveCatalogEditor(created.Path, renamed, created.Revision)
	if err != nil {
		t.Fatalf("a catalog is not renamed: %v", err)
	}
	if saved.Path != "metadata/catalogs/Склады/object.yaml" || saved.Catalog.Name != "Склады" {
		t.Errorf("the renamed catalog lies at %s named %s", saved.Path, saved.Catalog.Name)
	}
	if _, err := os.Stat(filepath.Join(root, "metadata", "catalogs", "Справочник1")); !os.IsNotExist(err) {
		t.Errorf("the old folder is still there: %v", err)
	}
	if _, err := metadata.Load(workspace.root); err != nil {
		t.Errorf("the project with the renamed catalog does not load: %v", err)
	}
	if _, err := workspace.ReadCatalogEditor(saved.Path); err != nil {
		t.Errorf("the renamed catalog does not open: %v", err)
	}
}

// TestTheTreeGivesTheWindowOfAnObject catches the page guessing the window
// of an object by the folder it lies in - «metadata/catalogs/…» in a regular
// expression, a flag for each kind - instead of the tree saying it: the branch
// of a kind with a window and every object of it carry its address, an
// object of a kind with none and the groups inside an object carry nothing.
func TestTheTreeGivesTheWindowOfAnObject(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	role, err := workspace.CreateRole("Кладовщик")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	// An object is found by its description, a branch and a group by the
	// tree's identifier of it.
	found := map[string]string{}
	var walk func(nodes []Node)
	walk = func(nodes []Node) {
		for _, node := range nodes {
			if node.Kind == "metadata" {
				found[node.Path] = node.Editor
			} else {
				found[node.ID] = node.Editor
			}
			walk(node.Children)
		}
	}
	walk(snapshot.Tree.Children)
	goods := "metadata/catalogs/Товары/object.yaml"
	place := map[string]string{
		"metadata/catalogs": "/api/catalog", goods: "/api/catalog", "metadata/catalogs/Контрагенты/object.yaml": "/api/catalog",
		"metadata/roles": "/api/role", role.Path: "/api/role",
		"metadata/documents": "/api/document", "metadata/documents/ПродажаТоваров/object.yaml": "/api/document",
		"metadata/accumulation-registers": "", "metadata/accumulation-registers/ОстаткиТоваров/object.yaml": "",
	}
	groups := 0
	for id := range found {
		if strings.HasSuffix(id, ":attributes") {
			place[id] = ""
			groups++
		}
	}
	if groups == 0 {
		t.Error("the tree shows no group «Реквизиты» to check")
	}
	for at, want := range place {
		if got, ok := found[at]; !ok || got != want {
			t.Errorf("the node %s gives the window %q (in the tree: %v), want %q", at, got, ok, want)
		}
	}
}

// TestTheProjectCarriesTheModelsLimits catches the description of ceilings
// Studio sets its fields by drifting from the model's: a name of 255, a code
// of 50, a description of 150, a border of 5, a scale of 999 (METADATA-OBJECTS.md,
// «Пределы на состав»; the border and the scale of a font - checked by the
// owner, 04.10.2026),
// written out here, not read from the model.
func TestTheProjectCarriesTheModelsLimits(t *testing.T) {
	t.Parallel()
	workspace, err := Open(copyDemoProjectForStudioTest(t))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspace.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if want := (metadata.EditorLimits{Name: 255, Code: 50, Description: 150, Levels: 10, BorderWidth: 5, FontScale: 999}); snapshot.Limits != want {
		t.Errorf("the project carries the limits %+v, want %+v", snapshot.Limits, want)
	}
}
