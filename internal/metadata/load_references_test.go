package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// A reference to an object that is not in the project is caught wherever it
// stands - a hand-edited file can put it anywhere. Each kind checks its own
// attributes on its own path, so each place is tried: an attribute of a chart
// of characteristic types and of its table part, of a chart of accounts, of a
// task and of an accounting register.
func TestAReferenceToNothingIsRefusedWhereverItStands(t *testing.T) {
	t.Parallel()
	attribute := func(name, missing string) string {
		return "  - {id: " + uuid.MustNew().String() + ", name: " + name + ", title: {ru: " + name + "}, types: [{kind: catalog, reference: " + missing + "}]}\n"
	}
	tests := []struct {
		name  string
		write func(t *testing.T, missing string) string
		want  string
	}{
		{"chart of characteristic types", func(t *testing.T, missing string) string {
			root := metadataProject(t)
			id := uuid.MustNew().String()
			writeMetadata(t, root, ChartOfCharacteristicTypesKind, id, "format: 1\nid: "+id+"\nname: Свойства\ntitle: {ru: Свойства}\n"+
				"code: {type: string, length: 9, auto: true}\ndescription_length: 100\nvalue_type: [{kind: boolean}]\n"+
				"attributes:\n"+attribute("Владелец", missing))
			return root
		}, "attribute Владелец references unknown catalog"},
		{"table part of a chart of characteristic types", func(t *testing.T, missing string) string {
			root := metadataProject(t)
			id := uuid.MustNew().String()
			writeMetadata(t, root, ChartOfCharacteristicTypesKind, id, "format: 1\nid: "+id+"\nname: Свойства\ntitle: {ru: Свойства}\n"+
				"code: {type: string, length: 9, auto: true}\ndescription_length: 100\nvalue_type: [{kind: boolean}]\n"+
				"table_parts:\n  - id: "+uuid.MustNew().String()+"\n    name: Значения\n    title: {ru: Значения}\n    attributes:\n"+
				strings.ReplaceAll(attribute("Значение", missing), "  - ", "      - "))
			return root
		}, "table part Значения attribute Значение references unknown catalog"},
		{"chart of accounts", func(t *testing.T, missing string) string {
			root := metadataProject(t)
			id := uuid.MustNew().String()
			writeMetadata(t, root, ChartOfAccountsKind, id, "format: 1\nid: "+id+"\nname: Основной\ntitle: {ru: Основной}\n"+
				"code: {type: string, length: 9, auto: false}\ndescription_length: 100\n"+
				"attributes:\n"+attribute("Подразделение", missing))
			return root
		}, "attribute Подразделение references unknown catalog"},
		{"task", func(t *testing.T, missing string) string {
			root := metadataProject(t)
			id := uuid.MustNew().String()
			writeMetadata(t, root, TaskKind, id, "format: 1\nid: "+id+"\nname: Задача\ntitle: {ru: Задача}\n"+
				"number: {type: string, length: 14, auto: true, unique: true, periodicity: none}\ndescription_length: 150\n"+
				"attributes:\n"+attribute("Клиент", missing))
			return root
		}, "attribute Клиент references unknown catalog"},
		{"accounting register", func(t *testing.T, missing string) string {
			return entriesProject(t, false, "resources:\n  - {id: "+uuid.MustNew().String()+", name: Сумма, title: {ru: Сумма}, types: [{kind: number, precision: 15, scale: 2}]}\n"+
				"attributes:\n"+attribute("Содержание", missing))
		}, "attribute Содержание references unknown catalog"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			missing := uuid.MustNew().String()
			root := test.write(t, missing)
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), test.want+" "+missing) {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

// A route point addresses its task by a value the addressing attribute has
// to be able to hold; a value of another type addresses nobody.
func TestARoutePointAddressedByAValueOfTheWrongTypeIsRefused(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, roleCatalogID, "format: 1\nid: "+roleCatalogID+"\nname: РолиИсполнителей\ntitle: {ru: Роли исполнителей}\n"+
		"code: {type: string, length: 9, auto: true}\ndescription_length: 100\n")
	writeMetadata(t, root, InformationRegisterKind, performersID, `format: 1
id: `+performersID+`
name: ИсполнителиЗадач
title: {ru: Исполнители задач}
write_mode: independent
periodicity: none
dimensions:
  - {id: b0000000-0000-4000-8000-000000000010, name: РольИсполнителя, title: {ru: Роль исполнителя}, types: [{kind: catalog, reference: `+roleCatalogID+`}]}
resources:
  - {id: b0000000-0000-4000-8000-000000000011, name: Исполнитель, title: {ru: Исполнитель}, types: [{kind: catalog, reference: `+roleCatalogID+`}]}
`)
	writeMetadata(t, root, TaskKind, taskID, `format: 1
id: `+taskID+`
name: ЗадачаИсполнителя
title: {ru: Задача исполнителя}
number: {type: string, length: 14, auto: true, unique: true, periodicity: none}
description_length: 150
addressing: `+performersID+`
main_addressing_attribute: РольИсполнителя
addressing_attributes:
  - {id: b0000000-0000-4000-8000-000000000020, name: РольИсполнителя, title: {ru: Роль исполнителя}, types: [{kind: catalog, reference: `+roleCatalogID+`}], dimension: b0000000-0000-4000-8000-000000000010}
`)
	writeMetadata(t, root, BusinessProcessKind, businessProcessID, `format: 1
id: `+businessProcessID+`
name: Задание
title: {ru: Задание}
number: {type: string, length: 11, auto: true, unique: true, periodicity: none}
task: `+taskID+`
route:
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - id: b0000000-0000-4000-8000-000000000031
      name: Выполнить
      kind: activity
      addressing:
        - {attribute: РольИсполнителя, value: {kind: string, data: Бухгалтер}}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Выполнить}
    - {from: Выполнить, to: Завершение}
`)
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "point Выполнить addressing РольИсполнителя") {
		t.Fatalf("Load() error = %v", err)
	}
}

// What lies in an object's folder is checked against what the object
// declares: a file nothing declares is a mistake in a data processor as much
// as anywhere else.
func TestADataProcessorFolderKeepsOnlyWhatItDeclares(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	id := uuid.MustNew().String()
	writeMetadata(t, root, DataProcessorKind, id, "format: 1\nid: "+id+"\nname: Заполнение\ntitle: {ru: Заполнение}\n")
	if err := os.WriteFile(filepath.Join(root, "metadata", string(DataProcessorKind), "Заполнение", "заметка.txt"), []byte("—"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "заметка.txt") {
		t.Fatalf("Load() error = %v", err)
	}
}

// Help keeps its pages and the folder of their pictures, nothing else; a
// subsystem's help is checked like any other.
func TestASubsystemHelpKeepsOnlyItsPages(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	id := uuid.MustNew().String()
	writeMetadata(t, root, SubsystemKind, id, "format: 1\nid: "+id+"\nname: Продажи\ntitle: {ru: Продажи}\n")
	help := filepath.Join(root, "metadata", string(SubsystemKind), id, project.HelpDirectory)
	if err := os.MkdirAll(help, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(help, "черновик.txt"), []byte("—"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), `keeps "черновик.txt" in its help`) {
		t.Fatalf("Load() error = %v", err)
	}
}
