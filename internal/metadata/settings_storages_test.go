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
	storageAuxSave  = "d3000000-0000-4000-8000-000000000005"
	storageAuxLoad  = "d3000000-0000-4000-8000-000000000006"
	storageReport   = "d3000000-0000-4000-8000-000000000010"
	storageMissing  = "d3000000-0000-4000-8000-0000000000ff"
)

// A settings storage keeps what the user saved: settings of forms, lists and
// reports, and variants of reports. It is shown through two forms - saving the
// current settings under a name, and picking a saved one - each with an
// auxiliary form beside it.
func TestSettingsStorageKeepsItsModuleAndItsFourForms(t *testing.T) {
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
  auxiliary_save: ВспомогательноеСохранение
  auxiliary_load: ВспомогательнаяЗагрузка
`)
	writeObjectModule(t, root, SettingsStorageKind, "ХранилищеВариантовОтчетов", project.ManagerModuleFile)
	for form, id := range map[string]string{
		"ФормаСохранения": storageSaveForm, "ФормаЗагрузки": storageLoadForm,
		"ВспомогательноеСохранение": storageAuxSave, "ВспомогательнаяЗагрузка": storageAuxLoad,
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
	case storage.Forms.AuxiliarySave == "" || storage.Forms.AuxiliaryLoad == "":
		t.Fatalf("the auxiliary forms were lost: %+v", storage.Forms)
	case storage.Comment == "":
		t.Fatalf("the comment was lost: %+v", storage)
	}

	storage.Forms.Save = ""
	again, _ := catalog.SettingsStorage("ХранилищеВариантовОтчетов")
	if again.Forms.Save == "" {
		t.Fatal("a settings storage was handed out by reference")
	}
}

// A report says where its variants and its settings are kept. Until this kind
// existed the name could not be resolved at all, so a report could point at a
// storage that was never there and nothing would say so.
func TestReportNamesAStorageThatExists(t *testing.T) {
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
	_, err := Load(root)
	if err == nil {
		t.Fatal("a report keeping its variants nowhere was accepted")
	}
	if !strings.Contains(err.Error(), "which is not in the configuration") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// An auxiliary form stands beside a main one, not behind it: the platform
// reaches for it exactly when the main form is missing or does not fit the
// client it has to be shown in. Naming one alone is therefore the ordinary
// case, and a storage that does so must be read, not refused.
func TestSettingsStorageKeepsAnAuxiliaryFormWithNoMainForm(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"только вспомогательная форма сохранения": "forms: {auxiliary_save: ВспомогательноеСохранение}",
		"только вспомогательная форма загрузки":   "forms: {auxiliary_load: ВспомогательнаяЗагрузка}",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeSettingsStorage("object.yaml", strings.NewReader(`format: 1
id: `+storageID+`
name: Хранилище
title: {ru: Хранилище}
`+body+`
`), metadataConfiguration()); err != nil {
				t.Fatalf("%s: refused: %v", name, err)
			}
		})
	}
}

// A role carries the name of a form, and a form is a folder named after
// itself, so anything that cannot be a folder name is not a form.
func TestSettingsStorageRefusesAFormThatIsNotAName(t *testing.T) {
	t.Parallel()
	_, err := DecodeSettingsStorage("object.yaml", strings.NewReader(`format: 1
id: `+storageID+`
name: Хранилище
title: {ru: Хранилище}
forms: {auxiliary_save: "Вспомогательное сохранение"}
`), metadataConfiguration())
	if err == nil {
		t.Fatal("a form name that cannot be a folder was accepted")
	}
	if !strings.Contains(err.Error(), "forms.auxiliary_save must be the name of a form") {
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
