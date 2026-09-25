package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

const commonFormID = "d7000000-0000-4000-8000-000000000001"

// writeCommonForm writes one common form into the folder named after it.
func writeCommonForm(t *testing.T, root, name, body string) {
	t.Helper()
	directory := filepath.Join(root, "metadata", "common-forms", name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, project.FormMetadataFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A common form belongs to no object, and everything that describes it used to
// be missing from the model: how it is built, which applications it is meant
// for, and the three ways it is described to a developer and to a user. A
// property the model does not carry is lost at import without a word.
func TestCommonFormKeepsWhatDescribesIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonForm(t, root, "АдреснаяКнига", `format: 1
id: `+commonFormID+`
name: АдреснаяКнига
title: {ru: Адресная книга}
kind: common
comment: Открывается из любого места конфигурации
explanation: {ru: Кому и куда писать}
extended_presentation: {ru: Адресная книга организации}
type: managed
purposes: [platform-application, mobile-platform-application]
use_standard_commands: true
include_help_in_contents: true
`)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
	forms, err := ReadCommonForms(root, metadataManifest())
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 1 {
		t.Fatalf("the common form was lost: %+v", forms)
	}
	form := forms[0]
	switch {
	case form.Comment == "":
		t.Fatalf("the comment was lost: %+v", form)
	case len(form.Explanation) == 0 || len(form.ExtendedPresentation) == 0:
		t.Fatalf("the presentations were lost: %+v", form)
	case form.Type != ManagedFormType:
		t.Fatalf("how the form is built was lost: %+v", form)
	case len(form.Purposes) != 2:
		t.Fatalf("what the form is meant for was lost: %+v", form.Purposes)
	case !form.UseStandardCommands || !form.IncludeHelpInContents:
		t.Fatalf("the two switches were lost: %+v", form)
	}
}

// A common form now lies the way every other form does: in a folder named
// after it, beside the module that runs it. Nothing about it is declared
// anywhere else, so the folder is what says the form exists at all - and it is
// read whether or not a role happens to refer to it.
func TestCommonFormLiesInAFolderOfItsOwn(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		write func(t *testing.T, root string)
		want  string
	}{
		"форма называет себя иначе, чем папка": {func(t *testing.T, root string) {
			writeCommonForm(t, root, "АдреснаяКнига", `format: 1
id: `+commonFormID+`
name: ДругаяФорма
title: {ru: Другая}
kind: common
`)
		}, "lies in a folder called"},
		"вместо папки файл": {func(t *testing.T, root string) {
			directory := filepath.Join(root, "metadata", "common-forms")
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, commonFormID+".yaml"), []byte("format: 1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "must be a folder"},
		"в папке формы лишний файл": {func(t *testing.T, root string) {
			writeCommonForm(t, root, "АдреснаяКнига", `format: 1
id: `+commonFormID+`
name: АдреснаяКнига
title: {ru: Адресная книга}
kind: common
`)
			stray := filepath.Join(root, "metadata", "common-forms", "АдреснаяКнига", "заметки.txt")
			if err := os.WriteFile(stray, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "a form keeps only its description and its module"},
		"папка без описания": {func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, "metadata", "common-forms", "АдреснаяКнига"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "has no " + project.FormMetadataFile},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			test.write(t, root)
			_, err := Load(root)
			if err == nil {
				t.Fatalf("%s: accepted", name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("%s: refused for another reason: %v", name, err)
			}
		})
	}
}

// The module of a common form lies beside it, the same as any other form's.
func TestCommonFormKeepsItsModuleBesideIt(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeCommonForm(t, root, "АдреснаяКнига", `format: 1
id: `+commonFormID+`
name: АдреснаяКнига
title: {ru: Адресная книга}
kind: common
`)
	module := filepath.Join(root, "metadata", "common-forms", "АдреснаяКнига", project.FormModuleFile)
	if err := os.WriteFile(module, []byte("&НаКлиенте\nПроцедура Открыть(Команда)\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
	expected, err := project.CommonFormModulePath("АдреснаяКнига")
	if err != nil {
		t.Fatal(err)
	}
	if expected != "metadata/common-forms/АдреснаяКнига/"+project.FormModuleFile {
		t.Fatalf("a common form's module lies at %q", expected)
	}
}

// What is checked about the new properties is what makes them mean anything.
func TestBrokenCommonFormPropertiesAreRefused(t *testing.T) {
	t.Parallel()
	for name, broken := range map[string]struct{ body, want string }{
		"вида формы не существует": {"type: рукописная", "type must be managed or ordinary"},
		"назначения не существует": {"purposes: [watch]", "purposes[0] is not a kind of application"},
		"назначение повторено": {"purposes: [platform-application, platform-application]",
			"purposes[1] is already among the purposes"},
		"пояснение на незаявленном языке": {"explanation: {de: Adressbuch}",
			"explanation.de uses an unconfigured language"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeManagedForm("form.yaml", strings.NewReader(`format: 1
id: `+commonFormID+`
name: АдреснаяКнига
title: {ru: Адресная книга}
kind: common
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
