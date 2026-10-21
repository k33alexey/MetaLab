package metadata

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
	"github.com/k33alexey/MetaLab/internal/project"
)

// The edges of block 2в the mutation pass of 21.10.2026 found no test on:
// each test below was red on the mutant that lived (2.432).

// TestAnIdentifierIsWrittenInWordsAsItIsWritten catches identifierWords
// breaking an identifier where the name does not: capitals written together
// are one word (XDTO, ОС), the word after them starts at its own capital, and
// a name ending in capitals ends with them, not with a fault reading past the
// last letter. The words are the ones the messages and the captions show.
func TestAnIdentifierIsWrittenInWordsAsItIsWritten(t *testing.T) {
	t.Parallel()
	for identifier, want := range map[string]string{
		"ChartOfAccounts": "chart of accounts",
		"XDTOPackage":     "XDTO package",
		"HTTPService":     "HTTP service",
		"ОбъектXDTO":      "объект XDTO",
		"ИзменениеАутентификацииОССеанса": "изменение аутентификации ОС сеанса",
		"": "",
	} {
		if got := identifierWords(identifier); got != want {
			t.Errorf("identifierWords(%q) = %q, want %q", identifier, got, want)
		}
	}
	// The caption the configurator gives a name with no synonym: the first
	// letter capital - and nothing at all for no name, not a fault.
	for name, want := range map[string]string{"ПометкаУдаления": "Пометка удаления", "": ""} {
		if got := nameInWords(name); got != want {
			t.Errorf("nameInWords(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestAStandardKeyTakesEveryLetterAndDigit catches the shape of a standard
// key refusing the last letter or the last digit: extdimension9 and a name
// with z are keys of the prototype's shape, a colon or a brace next to them
// are not.
func TestAStandardKeyTakesEveryLetterAndDigit(t *testing.T) {
	t.Parallel()
	for key, want := range map[string]bool{
		"extdimension9": true, "xyz": true, "a0": true,
		"a:": false, "a{": false, "a/": false, "a`": false, "9a": false, "": false,
	} {
		if got := standardKeyShape(key); got != want {
			t.Errorf("standardKeyShape(%q) = %v, want %v", key, got, want)
		}
	}
}

// TestATemplateNameMayBeAsLongAsAnyName catches the name of a template of
// restrictions refused at the length every other name may have: the limit is
// the platform's for a name (project.MaxNameLength), the one beyond it is
// refused.
func TestATemplateNameMayBeAsLongAsAnyName(t *testing.T) {
	t.Parallel()
	longest := strings.Repeat("Ш", project.MaxNameLength)
	if name, _, ok := templateName(longest + "(Поле)"); !ok || name != longest {
		t.Errorf("a template named with %d letters is refused", project.MaxNameLength)
	}
	if _, _, ok := templateName(longest + "Ш"); ok {
		t.Errorf("a template named with %d letters is taken", project.MaxNameLength+1)
	}
}

// TestACopiedStructIsWalkedAsItsOwn catches walkModel handing a visitor the
// fields of a struct it was given as a copy: a visitor takes what it reads by
// pointer (the holders of notes do), and a field of a copy has no address -
// the visitor would fail on the first one.
func TestACopiedStructIsWalkedAsItsOwn(t *testing.T) {
	t.Parallel()
	sample := walkSample{Title: LocalizedText{"ru": "Заголовок"}}
	walkModel(reflect.ValueOf(sample), "объект", func(value reflect.Value, where string) bool {
		if value.Kind() == reflect.Struct && !value.CanAddr() {
			t.Errorf("%s reached the visitor without an address", where)
		}
		return true
	})
}

// TestThisObjectNamesEveryFieldInBothLanguages catches the members of
// ЭтотОбъект losing the English name of a standard field: Наименование is
// Description in English (help, СправочникОбъект), and a field of the object,
// which has one name, is called by it in both.
func TestThisObjectNamesEveryFieldInBothLanguages(t *testing.T) {
	t.Parallel()
	catalog, definition := partedCatalog(t)
	english := map[string]string{}
	for _, member := range catalog.ThisObjectMembers()[project.ModuleAddress{Kind: string(CatalogKind), Object: definition.Name, Module: kinds.ObjectModule}] {
		english[member.Name.Russian] = member.Name.English
	}
	for russian, want := range map[string]string{"Наименование": "Description", "Код": "Code", "Цена": "Цена"} {
		if english[russian] != want {
			t.Errorf("ЭтотОбъект.%s is %q in English, want %q", russian, english[russian], want)
		}
	}
}

// TestTheSchemaOfRightsGivesPartsRequirementsAndBranches catches the schema
// the window of a role is built from losing what it says beside the rights:
// the parts of an object (an object with none gets an empty list, not
// nothing), the rights a right marks together with it - Администрирование
// marks Администрирование данных, Ввод по строке marks Чтение and Просмотр,
// Чтение marks nothing and is not listed (pass of 20.10.2026,
// configurator-2026-10-20/prava/zapis.md, «Карта зависимостей прав») -, and
// the branches: a catalog's with its rights, none for a kind no role gives
// rights on.
func TestTheSchemaOfRightsGivesPartsRequirementsAndBranches(t *testing.T) {
	t.Parallel()
	schema, err := LoadPermissionSchema(compositeProject(t))
	if err != nil {
		t.Fatal(err)
	}
	var withParts, withoutParts bool
	for _, object := range schema.Objects {
		parts, ok := schema.Parts[object.ID]
		if !ok {
			continue
		}
		if parts == nil {
			t.Errorf("%s has its parts as nothing, not as an empty list", object.Name)
		}
		if object.Kind == CatalogKind && len(parts) > 0 {
			withParts = true
		}
		if parts != nil && len(parts) == 0 {
			withoutParts = true
		}
	}
	if !withParts || !withoutParts {
		t.Errorf("the schema has a catalog with parts %v and an object without %v, want both", withParts, withoutParts)
	}
	for right, want := range map[PermissionOperation][]PermissionOperation{
		PermissionAdministration: {PermissionDataAdministration},
		PermissionInputByString:  {PermissionRead, PermissionView},
	} {
		for _, required := range want {
			if !slices.Contains(schema.Requires[right], required) {
				t.Errorf("%s does not mark %s: %v", right, required, schema.Requires[right])
			}
		}
	}
	if required, ok := schema.Requires[PermissionRead]; ok {
		t.Errorf("read is listed as marking %v, it marks nothing", required)
	}
	var catalog, module bool
	for _, branch := range schema.Branches {
		switch branch.Kind {
		case CatalogKind:
			catalog = slices.Contains(branch.Rights, PermissionRead)
		case CommonModuleKind:
			module = true
		}
	}
	if !catalog || module {
		t.Errorf("the branches have a catalog with read %v and common modules %v, want the first alone", catalog, module)
	}
}
