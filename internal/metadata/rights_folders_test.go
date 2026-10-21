package metadata

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
)

// folderNames writes the folders of the tree of rights as words, a sort of
// part by its group and the objects inside by their kind.
func folderNames(folders []rightsFolder) []string {
	var result []string
	for _, folder := range folders {
		if folder.kind != "" {
			result = append(result, "kind:"+string(folder.kind))
		} else {
			result = append(result, string(folder.group))
		}
	}
	return result
}

// The tree of rights shows under an object of every kind the folders the
// prototype's window of a role does, in its order, empty ones too (pass of
// 20.10.2026 with the owner, block-00/rol «Папки частей»,
// materials/configurator-2026-10-20/prava/zapis.md «Дерево прав»): written
// out here from the pass, not read off the descriptors. A chart of
// characteristic types, of calculation types, a data processor, a plan of
// exchange, a journal and a criterion of filter were opened by the pass
// after the series of 2.413 (snapshots prava/26-32); the table of
// dimensions of a cube was not - the copy has no external source - and is
// written by the nearest kind, a table of a source, standing apart until
// it is seen. Defects caught: one
// order for every kind (the standard attributes of a register of
// information after its attributes, the tables of dimensions of a cube
// last); the standard attributes or the commands missing; the recalculations
// of a register out of their place before the commands.
func TestTheTreeOfRightsShowsThePrototypesFolders(t *testing.T) {
	t.Parallel()
	reference := []string{"attributes", "table-parts", "standard-attributes", "commands"}
	register := []string{"dimensions", "resources", "attributes", "standard-attributes", "commands"}
	seen := map[Kind][]string{
		CatalogKind: reference, DocumentKind: reference, BusinessProcessKind: reference,
		TaskKind:   {"addressing-attributes", "attributes", "table-parts", "standard-attributes", "commands"},
		ReportKind: {"attributes", "table-parts", "commands"},
		ChartOfAccountsKind: {"attributes", "accounting-flags", "ext-dimension-accounting-flags", "table-parts",
			"standard-attributes", "standard-table-parts", "commands"},
		InformationRegisterKind:  {"dimensions", "resources", "standard-attributes", "attributes", "commands"},
		AccumulationRegisterKind: register, AccountingRegisterKind: register,
		CalculationRegisterKind:     {"dimensions", "resources", "attributes", "standard-attributes", "kind:recalculations", "commands"},
		ExternalDataSourceKind:      {"kind:external-data-source-tables", "kind:external-data-source-cubes", "kind:external-data-source-functions"},
		ExternalDataSourceTableKind: {"fields", "commands"},
		ExternalCubeKind:            {"kind:external-data-source-dimension-tables", "dimensions", "resources", "commands"},
		WebServiceKind:              {"operations"}, HTTPServiceKind: {"url-templates"},
		ChartOfCharacteristicTypesKind: reference, ExchangePlanKind: reference,
		// An enumeration shows its commands alone (pass of 21.10.2026,
		// prava/43).
		EnumerationKind:             {"commands"},
		ChartOfCalculationTypesKind: {"attributes", "table-parts", "standard-attributes", "standard-table-parts", "commands"},
		DataProcessorKind:           {"attributes", "table-parts", "commands"},
		DocumentJournalKind:         {"standard-attributes", "commands"},
		FilterCriterionKind:         {"commands"},
	}
	unseen := map[Kind][]string{ExternalDimensionTableKind: {"fields", "commands"}}
	for _, item := range kindInfos {
		want, ok := seen[item.kind]
		if !ok {
			want = unseen[item.kind]
		}
		if got := folderNames(kindCompositionOf[item.kind].rightsFolders); !slices.Equal(got, want) {
			t.Errorf("%s shows the folders %v, want %v", item.kind, got, want)
		}
	}
}

// Nothing a role gives rights on under an object is left out of its folders:
// every sort of part the kind's description keeps, its standard attributes
// and standard table parts, its commands, and every kind lying inside it
// that rights are given on - a folder missing would hide what it holds from
// the window of a role. Read off the other descriptors, so a kind that gains
// a sort of part is held here without a word. Defects caught: a kind gaining
// table parts, or a cube its tables of dimensions, with no folder for them.
func TestTheFoldersOfTheTreeOfRightsHoldEverythingAKindKeeps(t *testing.T) {
	t.Parallel()
	for _, item := range kindInfos {
		uses := usesOf(item.kind)
		if uses.offered == nil {
			continue
		}
		folders := folderNames(kindCompositionOf[item.kind].rightsFolders)
		var want []string
		if uses.parts != absent {
			for _, group := range branchesOf(item.kind) {
				want = append(want, string(group))
			}
			// The standard attributes of an enumeration take no rights: its
			// tree of rights shows its commands alone (prava/43).
			if len(standardFieldsOfKind(item.kind)) > 0 && item.kind != EnumerationKind {
				want = append(want, string(StandardAttributesPart))
			}
			if len(standardTablePartsOfKind(item.kind)) > 0 {
				want = append(want, string(StandardTablePartsPart))
			}
			if item.described != nil {
				if _, ok := item.described.FieldByName("Commands"); ok {
					want = append(want, string(CommandsPart))
				}
			}
		}
		for _, inner := range kinds.All() {
			if inner.Layout().Owner == item.kind && inner.Kind != item.kind && usesOf(inner.Kind).offered != nil {
				want = append(want, "kind:"+string(inner.Kind))
			}
		}
		for _, name := range want {
			if !slices.Contains(folders, name) {
				t.Errorf("%s keeps %s, which no folder of its tree of rights holds (%v)", item.kind, name, folders)
			}
		}
	}
}

// Every part an export gives rights on lies in a folder of its owner's kind:
// the Rights.xml of every role under docs/ names the part by its owner and
// its sort (Catalog.Товары.StandardAttribute.Код,
// ExternalDataSource.И.Table.Т.Field.П). The exports lie outside the
// repository; without them the test is skipped with the reason. Defect
// caught: a sort of part the exports give rights on with no folder in the
// tree of rights, which would hide it from the window of a role.
func TestTheFoldersOfTheTreeOfRightsHoldWhatTheExportsGiveRightsOn(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "docs")
	if _, err := os.Stat(filepath.Join(root, "import-examples")); err != nil {
		t.Skipf("the exports are not here (%v): they are kept out of the repository", err)
	}
	byClass := map[string]Kind{}
	for _, item := range kinds.All() {
		byClass[item.Name.English] = item.Kind
	}
	groupOf := map[string]PartGroup{}
	for _, sort := range partGroups {
		groupOf[sort.prototype] = sort.group
	}
	// The class of an object inside another, in a role's name of it.
	inner := map[string]Kind{"Table": ExternalDataSourceTableKind, "Cube": ExternalCubeKind,
		"DimensionTable": ExternalDimensionTableKind, "Recalculation": RecalculationKind}
	checked := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() != "Rights.xml" || !strings.Contains(filepath.ToSlash(path), "/Roles/") {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var role struct {
			Objects []struct {
				Name string `xml:"name"`
			} `xml:"object"`
		}
		if err := xml.Unmarshal(content, &role); err != nil {
			return err
		}
		for _, object := range role.Objects {
			parts := strings.Split(object.Name, ".")
			kind, ok := byClass[parts[0]]
			// A subsystem inside another is no part: the tree of rights shows
			// it under its parent as the project tree does.
			if !ok || len(parts) < 4 || kind == SubsystemKind {
				continue
			}
			// Walk down the name: each step is a class and a name, the
			// folder of the class taken in the kind the walk stands in.
			for at := 2; at+1 < len(parts); at += 2 {
				class := parts[at]
				name := string(kind) + " " + class
				if checked[name] {
					if next, ok := inner[class]; ok {
						kind = next
						continue
					}
					break
				}
				checked[name] = true
				folders := folderNames(kindCompositionOf[kind].rightsFolders)
				if next, ok := inner[class]; ok {
					if !slices.Contains(folders, "kind:"+string(next)) {
						t.Errorf("%s: %s gives rights inside %s, no folder of its kind holds %s", path, object.Name, kind, next)
					}
					kind = next
					continue
				}
				group, ok := groupOf[class]
				if !ok {
					t.Errorf("%s: %s names %s, no sort of part", path, object.Name, class)
					break
				}
				// A part inside a part - a field of a table part, a method of
				// a template - lies under that part, not in a folder.
				if !slices.Contains(folders, string(group)) {
					t.Errorf("%s: %s is a part of %s with no folder of its kind (%v)", path, object.Name, kind, folders)
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(checked) == 0 {
		t.Fatal("no part of an object in the exports")
	}
}
