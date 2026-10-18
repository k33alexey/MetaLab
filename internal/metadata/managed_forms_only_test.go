package metadata

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// managedOnlyProject is a report with one form and a common form, each with a
// module beside it; extra is written into both descriptions.
func managedOnlyProject(t *testing.T, extra string) (string, string, string) {
	t.Helper()
	root := metadataProject(t)
	report := uuid.MustNew().String()
	writeMetadata(t, root, ReportKind, report, "format: 1\nid: "+report+"\nname: Анализ\ntitle: {ru: Анализ}\n")
	object := filepath.Join(root, "metadata", string(ReportKind), "Анализ", "forms", "Форма")
	writeText(t, object, "", project.FormMetadataFile, "format: 1\nid: "+uuid.MustNew().String()+
		"\nname: Форма\ntitle: {ru: Форма}\nkind: object\n"+extra)
	writeText(t, object, "", project.FormModuleFile, "Процедура Обновить()\nКонецПроцедуры\n")
	writeCommonForm(t, root, "Настройки", "format: 1\nid: "+uuid.MustNew().String()+
		"\nname: Настройки\ntitle: {ru: Настройки}\nkind: common\n"+extra)
	common := filepath.Join(root, "metadata", "common-forms", "Настройки")
	writeText(t, common, "", project.FormModuleFile, "Процедура Обновить()\nКонецПроцедуры\n")
	return root, object, common
}

// ML takes managed forms only (owner, 14.10.2026, METADATA-OBJECTS «Принимаются
// только управляемые формы»): the body of an ordinary form has no place in a
// form's folder, an object's or a common one. Defect caught: the folder took
// form.bin, the body of a form nothing builds - an ordinary form carried into
// the project after all.
func TestFormFolderRefusesTheBodyOfAnOrdinaryForm(t *testing.T) {
	t.Parallel()
	for name, place := range map[string]int{"форма объекта": 1, "общая форма": 2} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, object, common := managedOnlyProject(t, "")
			directory := map[int]string{1: object, 2: common}[place]
			writeText(t, directory, "", "form.bin", "\xef\xbb\xbf{1,{...}}")
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), `keeps "form.bin"`) {
				t.Fatalf("the body of an ordinary form: refused for another reason or not at all: %v", err)
			}
		})
	}
}

// A form says nothing about what kind of form it is: every form is managed,
// and the property left with one value is gone with the ordinary form.
// Defect caught: a description still took "type", "ordinary" among its values.
func TestFormHasNoType(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"managed", "ordinary"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			root, _, _ := managedOnlyProject(t, "type: "+value+"\n")
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "type") {
				t.Fatalf("a form with type %s: refused for another reason or not at all: %v", value, err)
			}
		})
	}
}

// Every module of a form is compiled: none is carried aside as the module of
// an ordinary form. Defect caught: a form's module was left out of the
// compilation, and its errors with it, by a list of forms nothing builds.
func TestEveryFormModuleIsCompiled(t *testing.T) {
	t.Parallel()
	root, _, _ := managedOnlyProject(t, "")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	compiled := map[string]bool{}
	for _, module := range modules {
		compiled[module.Filename] = true
	}
	for _, path := range []string{
		"metadata/reports/Анализ/forms/Форма/" + project.FormModuleFile,
		"metadata/common-forms/Настройки/" + project.FormModuleFile,
	} {
		if !compiled[path] {
			t.Errorf("%s is not compiled", path)
		}
	}
}
