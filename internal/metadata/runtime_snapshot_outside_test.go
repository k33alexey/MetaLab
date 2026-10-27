package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// TestARuntimeSnapshotCarriesRightsAndMovementsOnKindsItHoldsNot holds the
// item of block 2в found by the pass 2.444 on stand С5: «Сохранить данные»
// builds the snapshot of execution from the kinds execution knows, and
// refused the whole project when a role gave a right on an object of another
// kind - a report, a chart, a business process - or a document wrote into a
// register of calculation or accounting. The composite project, each kind
// naming the others, gets a role giving every right offered on every object
// the schema of rights lists, a right on a part of a chart of accounts and a
// restriction of it. The snapshot keeps the role as the project has it and the
// movements of the document, the objects it holds no description of named in
// Outside, and so does the snapshot read back from its JSON, as ML App reads
// it - the registers of the movements before any role names them too; a
// right on, or a movement into, an object the project has not is still
// refused. Two other references of the composite project leave the snapshot
// as well - an attribute typed by a chart of characteristic types and the
// numerator of a document - and are taken out here: they are the item of
// block 3 on the references the snapshot loses. Defects caught: «role …
// references unknown or unsupported object» and «document … writes into …,
// which is not a register of the configuration» (2.444); the role or the
// movements cut down to what execution knows; the published snapshot refused
// on opening; anything at all accepted in place of an object of the project.
func TestARuntimeSnapshotCarriesRightsAndMovementsOnKindsItHoldsNot(t *testing.T) {
	t.Parallel()
	root := compositeProject(t)
	replace := func(kind Kind, name, old, new string) {
		t.Helper()
		path := filepath.Join(root, "metadata", string(kind), name, project.ObjectMetadataFile)
		content, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(content), old) {
			t.Fatalf("%s %s: %q not found (%v)", kind, name, old, err)
		}
		writeFile(t, path, strings.Replace(string(content), old, new, 1))
	}
	replace(CatalogKind, "Номенклатура", "  - {id: "+cmpGoodsProperty+", name: Свойство, title: {ru: Свойство}, types: [{kind: characteristic, reference: "+cmpCharacteristics+"}]}\n", "")
	replace(DocumentKind, "Поступление", "numerator: "+cmpNumerator, "number: {type: string, length: 11, auto: true, periodicity: year}")

	idOf := func(value string) uuid.UUID {
		id, err := uuid.Parse(value)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	// The registers of calculation and accounting the document writes into
	// are named by the movements alone, before any role names them.
	unroled, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	moved, err := NewRuntimeSnapshot(unroled, nil)
	if err != nil {
		t.Fatalf("the snapshot refuses the movements: %v", err)
	}
	if want := []RuntimeObject{{idOf(cmpEntries), AccountingRegisterKind}, {idOf(cmpPayroll), CalculationRegisterKind}}; !slices.Equal(moved.Outside, want) {
		t.Errorf("Outside of the movements is %v, want %v", moved.Outside, want)
	}
	schema, err := LoadPermissionSchema(root)
	if err != nil {
		t.Fatal(err)
	}
	role := RoleDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Всё", Title: LocalizedText{"ru": "Всё"}}
	for _, object := range schema.Objects {
		if object.Root || len(object.Rights) == 0 {
			continue
		}
		permission := ObjectPermission{Object: object.ID, Operations: object.Rights}
		if object.ID.String() == cmpAccounts {
			for _, field := range object.Fields {
				if len(field.Operations) > 0 {
					permission.Fields = []FieldPermission{{Field: field.Key, Operations: field.Operations}}
					break
				}
			}
			permission.Restrictions = []RightRestriction{{Right: PermissionRead, Fields: []string{"ref"}, Condition: "ИСТИНА"}}
		}
		role.Objects = append(role.Objects, permission)
	}
	if err := WriteRole(root, role); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewRuntimeSnapshot(catalog, nil)
	if err != nil {
		t.Fatalf("the snapshot refuses the project: %v", err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var published RuntimeSnapshot
	if err := json.Unmarshal(encoded, &published); err != nil {
		t.Fatal(err)
	}
	opened, err := published.Catalog()
	if err != nil {
		t.Fatalf("the published snapshot does not open: %v", err)
	}
	written, _ := catalog.RoleByID(role.ID)
	if len(written.Objects) < 20 {
		t.Fatalf("the role names %d objects, the composite project has more", len(written.Objects))
	}
	movements := []uuid.UUID{idOf(cmpPrices), idOf(cmpBalances), idOf(cmpEntries), idOf(cmpPayroll)}
	for name, held := range map[string]*Catalog{"the snapshot": nil, "the published snapshot": opened} {
		roles, documents := snapshot.Roles, snapshot.Documents
		if held != nil {
			roles, documents = held.Roles, held.Documents
		}
		index := slices.IndexFunc(roles, func(item RoleDefinition) bool { return item.ID == role.ID })
		if index < 0 || !reflect.DeepEqual(roles[index].Objects, written.Objects) {
			t.Errorf("%s does not keep the role as the project has it", name)
		}
		document := slices.IndexFunc(documents, func(item DocumentDefinition) bool { return item.Name == "Поступление" })
		if document < 0 || !slices.Equal(documents[document].Movements, movements) {
			t.Errorf("%s does not keep the movements of the document: %v", name, documents)
		}
	}
	outside := map[uuid.UUID]Kind{}
	for _, object := range published.Outside {
		outside[object.ID] = object.Kind
	}
	for id, kind := range map[string]Kind{cmpAccounts: ChartOfAccountsKind, cmpReport: ReportKind, cmpProcess: BusinessProcessKind,
		cmpEntries: AccountingRegisterKind, cmpPayroll: CalculationRegisterKind} {
		if outside[idOf(id)] != kind {
			t.Errorf("Outside names %s as %q, want %s", id, outside[idOf(id)], kind)
		}
	}
	if _, held := outside[idOf(cmpGoods)]; held {
		t.Error("Outside names a catalog the snapshot holds")
	}

	// What the project has not is refused as before.
	stranger := published
	stranger.Roles = slices.Clone(published.Roles)
	index := slices.IndexFunc(stranger.Roles, func(item RoleDefinition) bool { return item.ID == role.ID })
	stranger.Roles[index] = cloneRole(stranger.Roles[index])
	stranger.Roles[index].Objects = append(stranger.Roles[index].Objects, ObjectPermission{Object: uuid.MustNew(), Operations: []PermissionOperation{PermissionRead}})
	if _, err := stranger.Catalog(); err == nil || !strings.Contains(err.Error(), "unknown or unsupported object") {
		t.Errorf("a right on an object the project has not is accepted: %v", err)
	}
	stranger = published
	stranger.Documents = slices.Clone(published.Documents)
	document := slices.IndexFunc(stranger.Documents, func(item DocumentDefinition) bool { return item.Name == "Поступление" })
	stranger.Documents[document] = cloneDocumentDefinition(stranger.Documents[document])
	stranger.Documents[document].Movements = append(stranger.Documents[document].Movements, uuid.MustNew())
	if _, err := stranger.Catalog(); err == nil || !strings.Contains(err.Error(), "not a register of the project") {
		t.Errorf("a movement into an object the project has not is accepted: %v", err)
	}
}
