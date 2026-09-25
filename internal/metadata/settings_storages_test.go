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
  save: `+storageSaveForm+`
  load: `+storageLoadForm+`
  auxiliary_save: `+storageAuxSave+`
  auxiliary_load: `+storageAuxLoad+`
`)
	writeObjectModule(t, root, SettingsStorageKind, "ХранилищеВариантовОтчетов", project.ManagerModuleFile)
	for _, form := range []string{storageSaveForm, storageLoadForm, storageAuxSave, storageAuxLoad} {
		writeObjectForm(t, root, SettingsStorageKind, "ХранилищеВариантовОтчетов", form)
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
	case storage.Forms.Save == nil || storage.Forms.Load == nil:
		t.Fatalf("the main forms were lost: %+v", storage.Forms)
	case storage.Forms.AuxiliarySave == nil || storage.Forms.AuxiliaryLoad == nil:
		t.Fatalf("the auxiliary forms were lost: %+v", storage.Forms)
	case storage.Comment == "":
		t.Fatalf("the comment was lost: %+v", storage)
	}

	storage.Forms.Save = nil
	again, _ := catalog.SettingsStorage("ХранилищеВариантовОтчетов")
	if again.Forms.Save == nil {
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

// An auxiliary form stands beside a main one. Alone it is a form nothing ever
// opens, and saying so is cheaper than finding out from a user.
func TestBrokenSettingsStoragesAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"вспомогательная форма сохранения без основной": {"forms: {auxiliary_save: " + storageAuxSave + "}",
			"forms.auxiliary_save stands beside forms.save"},
		"вспомогательная форма загрузки без основной": {"forms: {auxiliary_load: " + storageAuxLoad + "}",
			"forms.auxiliary_load stands beside forms.load"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeSettingsStorage("object.yaml", strings.NewReader(`format: 1
id: `+storageID+`
name: Хранилище
title: {ru: Хранилище}
`+broken.body+`
`), metadataManifest())
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), broken.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
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
forms: {load: `+storageLoadForm+`}
`)
	if _, err := Load(root); err == nil {
		t.Fatal("a form that does not exist was accepted")
	}
	writeObjectForm(t, root, SettingsStorageKind, "Хранилище", storageLoadForm)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}
