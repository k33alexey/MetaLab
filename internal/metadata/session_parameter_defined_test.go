package metadata

import (
	"strings"
	"testing"
)

// A session parameter typed by a defined type is checked by the rules of a
// session parameter: «Проверка конфигурации» refuses one whose defined type
// holds a colour or holds «Произвольный» - «Недопустимый тип» - and passes an
// attribute of a data processor with the same defined types (opis of
// 23.10.2026, parametr-seansa.md, «Досъёмка»). The expectation is written here.
// Defects caught: the defined type checked by its own rules, which take both;
// a reference or a fixed array refused through the defined type; the data
// processor's attribute refused as well.
func TestASessionParameterThroughADefinedTypeKeepsItsOwnRules(t *testing.T) {
	t.Parallel()
	const (
		definedID   = "5d000000-0000-4000-8000-000000000001"
		parameterID = "5d000000-0000-4000-8000-000000000002"
		catalogID   = "5d000000-0000-4000-8000-000000000003"
		processorID = "5d000000-0000-4000-8000-000000000004"
	)
	for name, test := range map[string]struct{ types, refusal string }{
		"цвет":         {"[{kind: color}]", "session parameter Текущий: through its defined type"},
		"произвольный": {"[]", "its defined type holds no type"},
		"ссылка":       {"[{kind: catalog, reference: " + catalogID + "}]", ""},
		"фиксированный массив": {"[{kind: fixed-array}]", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, CatalogKind, catalogID, "format: 1\nid: "+catalogID+"\nname: Пользователи\ntitle: {ru: Пользователи}\n"+
				"code: {type: string, length: 9}\ndescription_length: 50\n")
			writeMetadata(t, root, DefinedTypeKind, definedID, "format: 1\nid: "+definedID+"\nname: Значение\ntitle: {ru: Значение}\ntypes: "+test.types+"\n")
			writeMetadata(t, root, SessionParameterKind, parameterID, "format: 1\nid: "+parameterID+"\nname: Текущий\ntitle: {ru: Текущий}\n"+
				"types: [{kind: defined-type, reference: "+definedID+"}]\n")
			_, err := Load(root)
			switch {
			case test.refusal == "" && err != nil:
				t.Fatalf("refused: %v", err)
			case test.refusal != "" && (err == nil || !strings.Contains(err.Error(), test.refusal)):
				t.Fatalf("error = %v, want one saying %q", err, test.refusal)
			}
		})
	}
	// The attribute of a data processor with a colour or «Произвольный»
	// through a defined type is taken: its own rules are wider.
	for name, types := range map[string]string{"цвет": "[{kind: color}]", "произвольный": "[]"} {
		t.Run("обработка, "+name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, DefinedTypeKind, definedID, "format: 1\nid: "+definedID+"\nname: Значение\ntitle: {ru: Значение}\ntypes: "+types+"\n")
			writeMetadata(t, root, DataProcessorKind, processorID, "format: 1\nid: "+processorID+"\nname: Обработка\ntitle: {ru: Обработка}\n"+
				"attributes:\n  - {id: 5d000000-0000-4000-8000-000000000005, name: Поле, title: {ru: Поле}, types: [{kind: defined-type, reference: "+definedID+"}]}\n")
			if _, err := Load(root); err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}
}
