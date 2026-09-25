package metadata

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestRuntimeSnapshotSortsAndIsolatesForms(t *testing.T) {
	t.Parallel()
	configuration := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"},
		DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	catalog, err := NewCatalogSnapshotWithAccumulationRegisters(configuration, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstID, err := uuid.Parse("10000000-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := uuid.Parse("20000000-0000-4000-8000-000000000002")
	if err != nil {
		t.Fatal(err)
	}
	forms := []ManagedForm{
		{Format: CurrentFormat, ID: secondID, Name: "Вторая", Title: LocalizedText{"ru": "Вторая"}, Kind: ObjectForm},
		{Format: CurrentFormat, ID: firstID, Name: "Первая", Title: LocalizedText{"ru": "Первая"}, Kind: ListForm},
	}
	snapshot, err := NewRuntimeSnapshot(catalog, forms)
	if err != nil {
		t.Fatal(err)
	}
	forms[0].Title["ru"] = "Изменена"
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	loaded, ok := snapshot.Form(secondID)
	if !ok || loaded.Title["ru"] != "Вторая" || snapshot.Forms[0].ID != firstID {
		t.Fatalf("snapshot=%+v loaded=%+v", snapshot, loaded)
	}
	loaded.Title["ru"] = "Ещё изменение"
	again, _ := snapshot.Form(secondID)
	if again.Title["ru"] != "Вторая" {
		t.Fatal("runtime form escaped snapshot isolation")
	}
}

func TestRuntimeSnapshotRejectsNonCanonicalFormOrder(t *testing.T) {
	t.Parallel()
	configuration := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"},
		DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	firstID, _ := uuid.Parse("10000000-0000-4000-8000-000000000001")
	secondID, _ := uuid.Parse("20000000-0000-4000-8000-000000000002")
	snapshot := RuntimeSnapshot{Format: CurrentFormat, Project: configuration, Forms: []ManagedForm{
		{Format: CurrentFormat, ID: secondID, Name: "Вторая", Title: LocalizedText{"ru": "Вторая"}, Kind: ObjectForm},
		{Format: CurrentFormat, ID: firstID, Name: "Первая", Title: LocalizedText{"ru": "Первая"}, Kind: ObjectForm},
	}}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("non-canonical runtime form order was accepted")
	}
}

func TestRuntimeSnapshotWithModulesSortsAndValidates(t *testing.T) {
	t.Parallel()
	configuration := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "Demo", Title: project.LocalizedText{"ru": "Demo"},
		DefaultLanguage: "ru", Languages: []project.Language{{ID: uuid.MustNew(), Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	catalog, err := NewCatalogSnapshotWithAccumulationRegisters(configuration, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewRuntimeSnapshot(catalog, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = snapshot.WithModules([]RuntimeModule{
		{Name: "ModuleB", Filename: "modules/second.bsl", Source: "// second"},
		{Name: "ModuleA", Filename: "modules/first.bsl", Source: "// first"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Modules) != 2 || snapshot.Modules[0].Name != "ModuleA" || snapshot.Modules[1].Name != "ModuleB" {
		t.Fatalf("modules were not sorted: %+v", snapshot.Modules)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot.WithModules([]RuntimeModule{{Name: "Дубль", Filename: "a.bsl"}, {Name: "Дубль", Filename: "b.bsl"}}); err == nil {
		t.Fatal("duplicate runtime module name was accepted")
	}
	if _, err := snapshot.WithModules([]RuntimeModule{{Name: "", Filename: "a.bsl"}}); err == nil {
		t.Fatal("runtime module without a name was accepted")
	}
}
