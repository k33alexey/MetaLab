package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// ordinaryFormProject is a report with one form of the given type, its body
// and its module beside it.
func ordinaryFormProject(t *testing.T, formType FormType) (string, string) {
	t.Helper()
	root := metadataProject(t)
	report := uuid.MustNew().String()
	writeMetadata(t, root, ReportKind, report, "format: 1\nid: "+report+"\nname: Анализ\ntitle: {ru: Анализ}\n")
	directory := filepath.Join(root, "metadata", string(ReportKind), "Анализ", "forms", "Форма")
	writeText(t, directory, "", project.FormMetadataFile, "format: 1\nid: "+uuid.MustNew().String()+
		"\nname: Форма\ntitle: {ru: Форма}\nkind: object\ntype: "+string(formType)+"\n")
	writeText(t, directory, "", project.FormBodyFile, "\xef\xbb\xbf{1,{...}}")
	writeText(t, directory, "", project.FormModuleFile, "Процедура КнопкаВыполнитьНажатие(Кнопка)\n\tСообщить(1);\nКонецПроцедуры\n")
	return root, directory
}

// An ordinary form keeps its body beside its description, as the prototype
// writes it, and its module beside both; the module is carried and not
// compiled, while the module of a managed form beside it is. Defects caught:
// the body was refused, and with it the report - 157, 160 and 1547 ordinary
// forms of the exports had nowhere to go, their code with them; or the module
// went into the compilation, where 1331 of 1864 such modules do not parse,
// and stopped the publication for code nobody runs.
func TestOrdinaryFormKeepsItsBodyAndItsModule(t *testing.T) {
	t.Parallel()
	root, _ := ordinaryFormProject(t, OrdinaryFormType)
	managed := filepath.Join(root, "metadata", string(ReportKind), "Анализ", "forms", "Управляемая")
	writeText(t, managed, "", project.FormMetadataFile, "format: 1\nid: "+uuid.MustNew().String()+
		"\nname: Управляемая\ntitle: {ru: Управляемая}\nkind: object\n")
	writeText(t, managed, "", project.FormModuleFile, "Процедура Обновить()\nКонецПроцедуры\n")
	for name, formType := range map[string]FormType{"Обычная": OrdinaryFormType, "Общая": ManagedFormType} {
		writeCommonForm(t, root, name, "format: 1\nid: "+uuid.MustNew().String()+"\nname: "+name+"\ntitle: {ru: "+name+"}\nkind: common\ntype: "+string(formType)+"\n")
		writeText(t, filepath.Join(root, "metadata", "common-forms", name), "", project.FormModuleFile, "Процедура Обновить()\nКонецПроцедуры\n")
	}
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("an ordinary form with its body was refused: %v", err)
	}
	modules, err := LoadProjectModules(root, catalog)
	if err != nil {
		t.Fatal(err)
	}
	compiled := map[string]bool{}
	for _, module := range modules {
		compiled[module.Filename] = true
	}
	for path, want := range map[string]bool{
		"metadata/reports/Анализ/forms/Форма/" + project.FormModuleFile:       false,
		"metadata/reports/Анализ/forms/Управляемая/" + project.FormModuleFile: true,
		"metadata/common-forms/Обычная/" + project.FormModuleFile:             false,
		"metadata/common-forms/Общая/" + project.FormModuleFile:               true,
	} {
		if compiled[path] != want {
			t.Errorf("%s compiled = %v, want %v", path, compiled[path], want)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Errorf("%s is not carried: %v", path, err)
		}
	}
}

// A body belongs to an ordinary form only, and is a file. Defects caught: a
// managed form took a body nobody reads - a second form beside the first;
// a folder took the place of the body.
func TestFormBodyBelongsToAnOrdinaryForm(t *testing.T) {
	t.Parallel()
	root, _ := ordinaryFormProject(t, ManagedFormType)
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "only an ordinary form has a body beside its description") {
		t.Fatalf("a managed form with a body: refused for another reason or not at all: %v", err)
	}
	root, directory := ordinaryFormProject(t, OrdinaryFormType)
	if err := os.Remove(filepath.Join(directory, project.FormBodyFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(directory, project.FormBodyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "which must be a file") {
		t.Fatalf("a folder for a body: refused for another reason or not at all: %v", err)
	}
}

// A common form is held to the same: an ordinary one keeps its body (energy,
// 49 common forms), a managed one does not. Defect caught: the common form's
// folder refused the body, or took it whatever the form was.
func TestCommonFormBodyBelongsToAnOrdinaryForm(t *testing.T) {
	t.Parallel()
	for formType, refusal := range map[FormType]string{
		OrdinaryFormType: "",
		ManagedFormType:  "only an ordinary form has a body beside its description",
	} {
		t.Run(string(formType), func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeCommonForm(t, root, "Настройки", "format: 1\nid: "+uuid.MustNew().String()+
				"\nname: Настройки\ntitle: {ru: Настройки}\nkind: common\ntype: "+string(formType)+"\n")
			writeText(t, filepath.Join(root, "metadata", "common-forms", "Настройки"), "", project.FormBodyFile, "{1}")
			_, err := Load(root)
			switch {
			case refusal == "" && err != nil:
				t.Fatalf("an ordinary common form with its body was refused: %v", err)
			case refusal != "" && (err == nil || !strings.Contains(err.Error(), refusal)):
				t.Fatalf("refused for another reason or not at all: %v", err)
			}
		})
	}
}
