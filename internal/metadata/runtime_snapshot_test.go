package metadata

import (
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestRuntimeSnapshotSortsAndIsolatesForms(t *testing.T) {
	t.Parallel()
	manifest := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "Demo", Title: "Demo",
		DefaultLanguage: "ru", Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	catalog, err := NewCatalogSnapshotWithAccumulationRegisters(manifest, nil, nil, nil, nil, nil, nil, nil)
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
	manifest := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "Demo", Title: "Demo",
		DefaultLanguage: "ru", Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	firstID, _ := uuid.Parse("10000000-0000-4000-8000-000000000001")
	secondID, _ := uuid.Parse("20000000-0000-4000-8000-000000000002")
	snapshot := RuntimeSnapshot{Format: CurrentFormat, Project: manifest, Forms: []ManagedForm{
		{Format: CurrentFormat, ID: secondID, Name: "Вторая", Title: LocalizedText{"ru": "Вторая"}, Kind: ObjectForm},
		{Format: CurrentFormat, ID: firstID, Name: "Первая", Title: LocalizedText{"ru": "Первая"}, Kind: ObjectForm},
	}}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("non-canonical runtime form order was accepted")
	}
}
