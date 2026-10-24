package metadata

import (
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const (
	storageID       = "d3000000-0000-4000-8000-000000000001"
	storageSaveForm = "d3000000-0000-4000-8000-000000000003"
	storageLoadForm = "d3000000-0000-4000-8000-000000000004"
	storageReport   = "d3000000-0000-4000-8000-000000000010"
	storageMissing  = "d3000000-0000-4000-8000-0000000000ff"
)

// A settings storage keeps what the user saved: settings of forms, lists and
// reports, and variants of reports. It is shown through two forms - saving the
// current settings under a name, and picking a saved one.
func TestSettingsStorageKeepsItsModuleAndItsTwoForms(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, SettingsStorageKind, storageID, `format: 1
id: `+storageID+`
name: ХранилищеВариантовОтчетов
title: {ru: Хранилище вариантов отчётов}
comment: Варианты отчётов, сохранённые пользователями
forms:
  save: ФормаСохранения
  load: ФормаЗагрузки
`)
	writeObjectModule(t, root, SettingsStorageKind, "ХранилищеВариантовОтчетов", project.ManagerModuleFile)
	for form, id := range map[string]string{
		"ФормаСохранения": storageSaveForm, "ФормаЗагрузки": storageLoadForm,
	} {
		writeObjectForm(t, root, SettingsStorageKind, "ХранилищеВариантовОтчетов", form, id)
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	storage, ok := catalog.SettingsStorage("ХранилищеВариантовОтчетов")
	if !ok {
		t.Fatal("the settings storage did not load")
	}
	switch {
	case storage.Forms.Save == "" || storage.Forms.Load == "":
		t.Fatalf("the main forms were lost: %+v", storage.Forms)
	case storage.Comment == "":
		t.Fatalf("the comment was lost: %+v", storage)
	}

	storage.Forms.Save = ""
	again, _ := catalog.SettingsStorage("ХранилищеВариантовОтчетов")
	if again.Forms.Save == "" {
		t.Fatal("a settings storage was handed out by reference")
	}
}

// A report says where its variants and its settings are kept. A storage that
// is there resolves; one that is gone is an error of the project. The strict load refuses the project naming the place; the reading for
// editing opens it and lists the reference to be fixed (owner, 05.10.2026).
func TestReportStorageThatIsGoneIsNamed(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, SettingsStorageKind, storageID, `format: 1
id: `+storageID+`
name: ХранилищеВариантов
title: {ru: Хранилище вариантов}
`)
	writeMetadata(t, root, ReportKind, storageReport, `format: 1
id: `+storageReport+`
name: Остатки
title: {ru: Остатки}
variants_storage: `+storageID+`
`)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}

	writeMetadata(t, root, ReportKind, storageReport, `format: 1
id: `+storageReport+`
name: Остатки
title: {ru: Остатки}
variants_storage: `+storageMissing+`
`)
	// A storage that is gone is an error of the project naming the place
	// (owner, 05.10.2026). Defect caught: the reference accepted silently,
	// or refused without saying where.
	if !hasUnresolved(t, root, "report Остатки variants storage", storageMissing) {
		t.Fatal("the storage that is gone is not named")
	}
}

// hasUnresolved reports whether the project refers by identifier to nothing
// at the given place: the strict load refuses it naming the place, and the
// reading for editing lists it.
func hasUnresolved(t *testing.T, root, where, id string) bool {
	t.Helper()
	for _, item := range unresolvedOf(t, root) {
		if item.Where == where && item.ID.String() == id {
			return true
		}
	}
	return false
}

// A role carries the name of a form, and a form is a folder named after
// itself, so anything that cannot be a folder name is not a form.
func TestSettingsStorageRefusesAFormThatIsNotAName(t *testing.T) {
	t.Parallel()
	_, err := DecodeSettingsStorage("object.yaml", strings.NewReader(`format: 1
id: `+storageID+`
name: Хранилище
title: {ru: Хранилище}
forms: {save: "Форма сохранения"}
`), metadataConfiguration())
	if err == nil {
		t.Fatal("a form name that cannot be a folder was accepted")
	}
	if !strings.Contains(err.Error(), "forms.save must be the name of a form") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// A form the storage declares has to exist, and it is looked for inside the
// storage's own folder.
func TestSettingsStorageFormsAreExpectedInItsFolder(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, SettingsStorageKind, storageID, `format: 1
id: `+storageID+`
name: Хранилище
title: {ru: Хранилище}
forms: {load: ФормаЗагрузки}
`)
	if _, err := Load(root); err == nil {
		t.Fatal("a form that does not exist was accepted")
	}
	writeObjectForm(t, root, SettingsStorageKind, "Хранилище", "ФормаЗагрузки", storageLoadForm)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}
