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
	decoded, err := DecodeSubsystem("subsystem.yaml", bytes.NewReader(encoded.Bytes()), metadataManifest())
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
			if err := ValidateSubsystem("subsystem.yaml", mutated, metadataManifest()); err == nil {
				t.Fatalf("%s: accepted invalid subsystem", name)
			}
		})
	}
	if _, err := DecodeSubsystem("subsystem.yaml", strings.NewReader(encoded.String()+"unknown: true\n"), metadataManifest()); err == nil {
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
	catalog, err := NewCatalogSnapshotWithSubsystems(metadataManifest(), nil, nil, nil, []CatalogDefinition{member}, nil, nil, nil, nil, []SubsystemDefinition{subsystem})
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
	if _, err := load(root, true); err == nil {
		t.Fatal("subsystem with an unknown member was accepted")
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
