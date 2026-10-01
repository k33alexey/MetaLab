package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func subsystemFixture() SubsystemDefinition {
	return SubsystemDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Продажи", Title: LocalizedText{"ru": "Продажи"}}
}

func TestDecodeSubsystemStrictAndBounded(t *testing.T) {
	t.Parallel()
	subsystem := subsystemFixture()
	subsystem.Members = []uuid.UUID{uuid.MustNew(), uuid.MustNew()}
	var encoded bytes.Buffer
	if err := Encode(&encoded, subsystem); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSubsystem("subsystem.yaml", bytes.NewReader(encoded.Bytes()), metadataConfiguration())
	if err != nil || decoded.ID != subsystem.ID || len(decoded.Members) != 2 {
		t.Fatalf("decode subsystem = %+v, %v", decoded, err)
	}
	tests := map[string]func(*SubsystemDefinition){
		"format":           func(s *SubsystemDefinition) { s.Format++ },
		"zero identity":    func(s *SubsystemDefinition) { s.ID = uuid.UUID{} },
		"name":             func(s *SubsystemDefinition) { s.Name = "Invalid Name" },
		"language":         func(s *SubsystemDefinition) { s.Title = LocalizedText{"de": "Verkauf"} },
		"self parent":      func(s *SubsystemDefinition) { self := s.ID; s.Parent = &self },
		"zero parent":      func(s *SubsystemDefinition) { zero := uuid.UUID{}; s.Parent = &zero },
		"zero member":      func(s *SubsystemDefinition) { s.Members = append(s.Members, uuid.UUID{}) },
		"duplicate member": func(s *SubsystemDefinition) { s.Members = append(s.Members, s.Members[0]) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			mutated := subsystem
			mutated.Members = append([]uuid.UUID{}, subsystem.Members...)
			mutate(&mutated)
			if err := ValidateSubsystem("subsystem.yaml", mutated, metadataConfiguration()); err == nil {
				t.Fatalf("%s: accepted invalid subsystem", name)
			}
		})
	}
	if _, err := DecodeSubsystem("subsystem.yaml", strings.NewReader(encoded.String()+"unknown: true\n"), metadataConfiguration()); err == nil {
		t.Fatal("accepted unknown field")
	}
}

func TestCatalogSubsystemLookupReturnsIsolatedCopies(t *testing.T) {
	t.Parallel()
	member := CatalogDefinition{Format: CurrentFormat, ID: uuid.MustNew(), Name: "Товары", Title: LocalizedText{"ru": "Товары"},
		Code: CatalogCode{Type: StringType, Length: 9}, DescriptionLength: 100}
	memberID := member.ID
	subsystem := subsystemFixture()
	subsystem.Members = []uuid.UUID{memberID}
	catalog, err := NewCatalogSnapshotWithSubsystems(metadataConfiguration(), nil, nil, nil, []CatalogDefinition{member}, nil, nil, nil, nil, []SubsystemDefinition{subsystem})
	if err != nil {
		t.Fatal(err)
	}
	byName, ok := catalog.Subsystem("Продажи")
	if !ok || byName.ID != subsystem.ID {
		t.Fatalf("subsystem by name = %+v, %v", byName, ok)
	}
	byID, ok := catalog.SubsystemByID(subsystem.ID)
	if !ok || byID.Name != "Продажи" {
		t.Fatalf("subsystem by id = %+v, %v", byID, ok)
	}
	byID.Members[0] = uuid.MustNew()
	again, _ := catalog.SubsystemByID(subsystem.ID)
	if again.Members[0] != memberID {
		t.Fatal("subsystem lookup exposed mutable metadata")
	}
}

func TestLoadValidatesSubsystemReferences(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	if err := os.MkdirAll(filepath.Join(root, "metadata", string(SubsystemKind)), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogID := uuid.MustNew()
	writeMetadata(t, root, CatalogKind, catalogID.String(), "format: 1\nid: "+catalogID.String()+"\nname: Товары\ntitle: {ru: Товары}\ncode: {type: string, length: 9}\ndescription_length: 100\n")

	validID, parentID := uuid.MustNew(), uuid.MustNew()
	writeMetadata(t, root, SubsystemKind, parentID.String(), "format: 1\nid: "+parentID.String()+"\nname: Продажи\ntitle: {ru: Продажи}\n")
	writeMetadata(t, root, SubsystemKind, validID.String(), "format: 1\nid: "+validID.String()+"\nname: ПродажиРозница\ntitle: {ru: Розница}\nparent: "+parentID.String()+"\nmembers: ["+catalogID.String()+"]\n")
	if _, err := load(root, true); err != nil {
		t.Fatalf("valid subsystem tree rejected: %v", err)
	}

	unknownMemberID := uuid.MustNew()
	if err := os.WriteFile(filepath.Join(root, "metadata", string(SubsystemKind), validID.String()+".yaml"),
		[]byte("format: 1\nid: "+validID.String()+"\nname: ПродажиРозница\ntitle: {ru: Розница}\nmembers: ["+unknownMemberID.String()+"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A member naming nothing is a deleted object the subsystem kept: carried,
	// and listed for the import report rather than refusing the configuration.
	catalog, err := load(root, true)
	if err != nil {
		t.Fatalf("subsystem with a stale member refused: %v", err)
	}
	if unresolved := catalog.UnresolvedReferences(); len(unresolved) != 1 || unresolved[0].ID != unknownMemberID {
		t.Fatalf("unresolved = %+v, want the stale member", unresolved)
	}

	// Two subsystems whose parents point at each other must be rejected.
	if err := os.WriteFile(filepath.Join(root, "metadata", string(SubsystemKind), validID.String()+".yaml"),
		[]byte("format: 1\nid: "+validID.String()+"\nname: ПродажиРозница\ntitle: {ru: Розница}\nparent: "+parentID.String()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metadata", string(SubsystemKind), parentID.String()+".yaml"),
		[]byte("format: 1\nid: "+parentID.String()+"\nname: Продажи\ntitle: {ru: Продажи}\nparent: "+validID.String()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := load(root, true); err == nil {
		t.Fatal("cyclical subsystem parent chain was accepted")
	}

	if err := os.WriteFile(filepath.Join(root, "metadata", string(SubsystemKind), parentID.String()+".yaml"),
		[]byte("format: 1\nid: "+parentID.String()+"\nname: Продажи\ntitle: {ru: Продажи}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := load(root, true); err != nil {
		t.Fatalf("valid parent chain rejected after repair: %v", err)
	}
}

// A subsystem holds objects of any kind of the top level - the prototype puts
// some forty kinds in, common modules, roles and pictures the most - and other
// subsystems too, not only its own children.
//
// Defect caught: a member of a kind outside the old six (catalog, document,
// enumeration, constant, two registers) taken for a member pointing at
// nothing - before this it refused the configuration, now it would be listed as
// stale.
func TestSubsystemHoldsObjectsOfEveryKind(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	for _, directory := range []string{"subsystems", "roles"} {
		if err := os.MkdirAll(filepath.Join(root, "metadata", directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reportID, roleID, otherID, ownerID := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	writeMetadata(t, root, ReportKind, reportID.String(), "format: 1\nid: "+reportID.String()+"\nname: Ведомость\ntitle: {ru: Ведомость}\n")
	if err := os.WriteFile(filepath.Join(root, "metadata", "roles", roleID.String()+".yaml"),
		[]byte("format: 1\nid: "+roleID.String()+"\nname: Бухгалтер\ntitle: {ru: Бухгалтер}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCommonForm(t, root, "АдреснаяКнига", "format: 1\nid: "+commonFormID+"\nname: АдреснаяКнига\ntitle: {ru: Адресная книга}\nkind: common\n")
	writeMetadata(t, root, SubsystemKind, otherID.String(), "format: 1\nid: "+otherID.String()+"\nname: Закупки\ntitle: {ru: Закупки}\n")
	writeMetadata(t, root, SubsystemKind, ownerID.String(), "format: 1\nid: "+ownerID.String()+"\nname: Продажи\ntitle: {ru: Продажи}\nmembers: ["+
		reportID.String()+", "+roleID.String()+", "+commonFormID+", "+otherID.String()+"]\n")
	catalog, err := load(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if unresolved := catalog.UnresolvedReferences(); len(unresolved) != 0 {
		t.Fatalf("members of other kinds taken for stale ones: %+v", unresolved)
	}
}

// A subsystem's name is unique among its siblings, not across the
// configuration: БазоваяФункциональность stands under a dozen parents in each
// of the configurations being moved. Two siblings of one name are still one
// name too many, and the lookup goes by path.
//
// Defect caught: the same name under two parents refused (41/35/29 repeats);
// two siblings of one name accepted; a nested subsystem found by its bare name,
// which is ambiguous.
func TestSubsystemNameIsUniqueAmongItsSiblings(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	if err := os.MkdirAll(filepath.Join(root, "metadata", string(SubsystemKind)), 0o755); err != nil {
		t.Fatal(err)
	}
	sales, purchases, first, second := uuid.MustNew(), uuid.MustNew(), uuid.MustNew(), uuid.MustNew()
	write := func(id uuid.UUID, name string, parent *uuid.UUID) {
		body := "format: 1\nid: " + id.String() + "\nname: " + name + "\ntitle: {ru: " + name + "}\n"
		if parent != nil {
			body += "parent: " + parent.String() + "\n"
		}
		writeMetadata(t, root, SubsystemKind, id.String(), body)
	}
	write(sales, "Продажи", nil)
	write(purchases, "Закупки", nil)
	write(first, "БазоваяФункциональность", &sales)
	write(second, "БазоваяФункциональность", &purchases)
	catalog, err := load(root, true)
	if err != nil {
		t.Fatalf("one name under two parents refused: %v", err)
	}
	if found, ok := catalog.Subsystem("продажи.базоваяфункциональность"); !ok || found.ID != first {
		t.Fatalf("lookup by path = %+v, %v", found, ok)
	}
	if found, ok := catalog.Subsystem("Закупки.БазоваяФункциональность"); !ok || found.ID != second {
		t.Fatalf("lookup by path = %+v, %v", found, ok)
	}
	if _, ok := catalog.Subsystem("БазоваяФункциональность"); ok {
		t.Fatal("a nested subsystem was found by its bare name")
	}

	write(second, "БазоваяФункциональность", &sales)
	if _, err := load(root, true); err == nil || !strings.Contains(err.Error(), "БазоваяФункциональность") {
		t.Fatalf("two siblings of one name: %v", err)
	}
}
