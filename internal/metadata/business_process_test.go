package metadata

import (
	"fmt"
	"testing"
)

const (
	taskID            = "b0000000-0000-4000-8000-000000000001"
	businessProcessID = "b0000000-0000-4000-8000-000000000002"
	performersID      = "b0000000-0000-4000-8000-000000000003"
	roleCatalogID     = "b0000000-0000-4000-8000-000000000004"
	roleValueID       = "b0000000-0000-4000-8000-000000000050"
)

// A route drawn without its branches keeps its lines and loses what decides
// between them, and a task without its addressing reaches nobody. Both travel
// or neither is transferred.
func TestLoadBusinessProcessWithRouteAndAddressedTask(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, roleCatalogID, `format: 1
id: `+roleCatalogID+`
name: РолиИсполнителей
title: {ru: Роли исполнителей}
code: {type: string, length: 9, auto: true}
description_length: 100
`)
	writeMetadata(t, root, InformationRegisterKind, performersID, `format: 1
id: `+performersID+`
name: ИсполнителиЗадач
title: {ru: Исполнители задач}
write_mode: independent
periodicity: none
dimensions:
  - id: b0000000-0000-4000-8000-000000000010
    name: РольИсполнителя
    title: {ru: Роль исполнителя}
    types: [{kind: catalog, reference: `+roleCatalogID+`}]
resources:
  - id: b0000000-0000-4000-8000-000000000011
    name: Исполнитель
    title: {ru: Исполнитель}
    types: [{kind: catalog, reference: `+roleCatalogID+`}]
`)
	writeMetadata(t, root, TaskKind, taskID, `format: 1
id: `+taskID+`
name: ЗадачаИсполнителя
title: {ru: Задача исполнителя}
number: {type: string, length: 14, auto: true, unique: true, periodicity: none}
description_length: 150
number_prefix: business-process-number
addressing: `+performersID+`
main_addressing_attribute: РольИсполнителя
addressing_attributes:
  - id: b0000000-0000-4000-8000-000000000020
    name: РольИсполнителя
    title: {ru: Роль исполнителя}
    types: [{kind: catalog, reference: `+roleCatalogID+`}]
    dimension: b0000000-0000-4000-8000-000000000010
`)
	writeMetadata(t, root, BusinessProcessKind, businessProcessID, `format: 1
id: `+businessProcessID+`
name: Задание
title: {ru: Задание}
number: {type: string, length: 11, auto: true, unique: true, periodicity: none}
task: `+taskID+`
create_tasks_privileged: true
route:
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start, location: {top: 20, left: 380, bottom: 60, right: 420}}
    - id: b0000000-0000-4000-8000-000000000031
      name: Выполнить
      kind: activity
      task_description: Выполнить
      group: false
      location: {top: 100, left: 340, bottom: 160, right: 460}
      addressing:
        - attribute: РольИсполнителя
          value: {kind: catalog, data: `+roleValueID+`}
    - {id: b0000000-0000-4000-8000-000000000032, name: НужнаПроверка, kind: condition}
    - {id: b0000000-0000-4000-8000-000000000033, name: Проверить, kind: activity, task_description: Проверить, addressing: [{attribute: РольИсполнителя}]}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Выполнить, vertices: [{x: 400, y: 60}, {x: 400, y: 100}]}
    - {from: Выполнить, to: НужнаПроверка}
    - {from: НужнаПроверка, to: Проверить, branch: "true"}
    - {from: НужнаПроверка, to: Завершение, branch: "false"}
    - {from: Проверить, to: Завершение}
  decorations:
    - {name: Декорация1, title: {ru: постановка задачи}, shape: Document, location: {top: 100, left: 20, bottom: 160, right: 140}}
    - {name: Черта, line: [{x: 20, y: 200}, {x: 300, y: 200}]}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	process, ok := catalog.BusinessProcess("Задание")
	if !ok {
		t.Fatal("the business process did not load")
	}
	if len(process.Route.Points) != 5 || len(process.Route.Transitions) != 5 {
		t.Fatalf("the route lost its shape: %+v", process.Route)
	}
	if !process.CreateTasksPrivileged || process.Task == nil {
		t.Fatalf("the process lost how it creates tasks: %+v", process)
	}
	var branches int
	for _, transition := range process.Route.Transitions {
		if transition.Branch != "" {
			branches++
		}
	}
	if branches != 2 {
		t.Fatalf("a condition must keep both of its branches, got %d", branches)
	}
	// The drawing travels whole: a map laid out afresh by the machine is not
	// the map the developer drew.
	if len(process.Route.Decorations) != 2 {
		t.Fatalf("the decorations of the map were lost: %+v", process.Route.Decorations)
	}
	if process.Route.Points[0].Location == nil || process.Route.Points[0].Location.Left != 380 {
		t.Fatalf("a point lost where it is drawn: %+v", process.Route.Points[0])
	}
	if len(process.Route.Transitions[0].Vertices) != 2 {
		t.Fatalf("a line lost its corners: %+v", process.Route.Transitions[0])
	}
	var addressedPoints int
	for _, point := range process.Route.Points {
		if len(point.Addressing) > 0 {
			addressedPoints++
		}
	}
	if addressedPoints != 2 {
		t.Fatalf("the points lost what they address their tasks by, got %d", addressedPoints)
	}
	task, ok := catalog.Task("ЗадачаИсполнителя")
	if !ok {
		t.Fatal("the task did not load")
	}
	if task.MainAddressingAttribute != "РольИсполнителя" || len(task.AddressingAttributes) != 1 {
		t.Fatalf("addressing was lost: %+v", task)
	}
	if task.AddressingAttributes[0].Dimension == nil {
		t.Fatal("an addressing attribute matched against nothing addresses nobody")
	}
	if task.NumberPrefix != BusinessProcessNumberPrefix {
		t.Fatalf("the number prefix was lost: %s", task.NumberPrefix)
	}

	schema, err := catalog.ApplicationSchema()
	if err != nil {
		t.Fatal(err)
	}
	taskTable, err := PhysicalCatalogTable(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var columns map[string]bool
	for _, table := range schema.Tables {
		if table.Name == taskTable {
			columns = map[string]bool{}
			for _, column := range table.Columns {
				columns[column.Name] = true
			}
		}
	}
	for _, column := range []string{"executed", "business_process", "route_point", "description"} {
		if !columns[column] {
			t.Fatalf("the task table has no %s", column)
		}
	}
}

// A map that cannot be walked is not a map: no start, no way to finish, a step
// nobody can reach, a branch out of a condition that does not say which way it
// goes.
func TestRouteRefusesAMapThatCannotBeWalked(t *testing.T) {
	t.Parallel()
	for name, route := range map[string]string{
		"без старта": `
  points:
    - {id: b0000000-0000-4000-8000-000000000031, name: Выполнить, kind: activity}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Выполнить, to: Завершение}`,
		"без завершения": `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000031, name: Выполнить, kind: activity}
  transitions:
    - {from: Старт, to: Выполнить}`,
		"недостижимая точка": `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
    - {id: b0000000-0000-4000-8000-000000000031, name: Забытая, kind: activity}
  transitions:
    - {from: Старт, to: Завершение}`,
		"условие без ветки": `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000032, name: Проверка, kind: condition}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Проверка}
    - {from: Проверка, to: Завершение}`,
		"переход в никуда": `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}
    - {from: Завершение, to: Невидимка}`,
		"выход из завершения": `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
    - {id: b0000000-0000-4000-8000-000000000031, name: Ещё, kind: activity}
  transitions:
    - {from: Старт, to: Завершение}
    - {from: Завершение, to: Ещё}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, TaskKind, taskID, `format: 1
id: `+taskID+`
name: ЗадачаИсполнителя
title: {ru: Задача исполнителя}
number: {type: string, length: 14, auto: true, unique: true, periodicity: none}
description_length: 150
`)
			writeMetadata(t, root, BusinessProcessKind, businessProcessID, `format: 1
id: `+businessProcessID+`
name: Задание
title: {ru: Задание}
number: {type: string, length: 11, auto: true, unique: true, periodicity: none}
task: `+taskID+`
route:`+route+`
`)
			if _, err := Load(root); err == nil {
				t.Fatal("a route that cannot be walked was accepted")
			}
		})
	}
}

// Addressing is the whole point of a task: a step that addresses its tasks by
// an attribute the kind of task does not have reaches nobody, and does it
// silently. A task number that restarts with the year tears its process in
// two. A decoration drawn nowhere is not a decoration.
func TestBusinessProcessRefusesAddressingAndNumberingThatMeanNothing(t *testing.T) {
	t.Parallel()
	const task = `format: 1
id: ` + taskID + `
name: ЗадачаИсполнителя
title: {ru: Задача исполнителя}
number: {type: string, length: 14, auto: true, unique: true, periodicity: %s}
description_length: 150
`
	for name, broken := range map[string]struct{ task, route string }{
		"адресация по несуществующему реквизиту": {"none", `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000031, name: Выполнить, kind: activity, addressing: [{attribute: Исполнитель}]}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Выполнить}
    - {from: Выполнить, to: Завершение}`},
		"адресация на точке, не создающей задач": {"none", `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000032, name: Проверка, kind: condition, addressing: [{attribute: Исполнитель}]}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Проверка}
    - {from: Проверка, to: Завершение, branch: "true"}
    - {from: Проверка, to: Завершение, branch: "false"}`},
		"периодичный номер задачи": {"year", `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}`},
		"декорация, нарисованная нигде": {"none", `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}
  decorations:
    - {name: Ничто, title: {ru: Ничто}}`},
		"точка, нарисованная наизнанку": {"none", `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start, location: {top: 60, left: 420, bottom: 20, right: 380}}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeMetadata(t, root, TaskKind, taskID, fmt.Sprintf(task, broken.task))
			writeMetadata(t, root, BusinessProcessKind, businessProcessID, `format: 1
id: `+businessProcessID+`
name: Задание
title: {ru: Задание}
number: {type: string, length: 11, auto: true, unique: true, periodicity: none}
task: `+taskID+`
route:`+broken.route+`
`)
			if _, err := Load(root); err == nil {
				t.Fatal("a process that addresses or numbers nothing was accepted")
			}
		})
	}
}

// A reference to a route point is carried by name. The name is only meaningful
// inside the map that drew it, so a value naming a point no map has must be
// refused when it is written, not discovered years later as a filter that
// never matches.
func TestRoutePointValueIsCheckedAgainstTheMap(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, TaskKind, taskID, `format: 1
id: `+taskID+`
name: ЗадачаИсполнителя
title: {ru: Задача исполнителя}
number: {type: string, length: 14, auto: true, unique: true, periodicity: none}
description_length: 150
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
    - {id: b0000000-0000-4000-8000-000000000031, name: Выполнить, kind: activity}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Выполнить}
    - {from: Выполнить, to: Завершение}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	process, ok := catalog.BusinessProcess("Задание")
	if !ok {
		t.Fatal("the business process did not load")
	}
	types := []Type{{Kind: RoutePointType, Reference: &process.ID}}
	normalized, err := catalog.normalizeTypes("attribute Шаг", types, Value{Kind: RoutePointType, Data: "выполнить"})
	if err != nil {
		t.Fatalf("a point of the map was refused: %v", err)
	}
	// The name is stored the way the map spells it, not the way it was typed.
	if normalized.Data != "Выполнить" {
		t.Fatalf("the point was not stored as the map spells it: %q", normalized.Data)
	}
	if _, err := catalog.normalizeTypes("attribute Шаг", types, Value{Kind: RoutePointType, Data: "Небывалая"}); err == nil {
		t.Fatal("a point no map has was accepted")
	}
	// A task that has not reached a point yet stands nowhere, and nowhere is a
	// value it must be able to hold.
	if _, err := catalog.normalizeTypes("attribute Шаг", types, Value{Kind: RoutePointType}); err != nil {
		t.Fatalf("the empty point was refused: %v", err)
	}
}

// An addressing attribute holding what its dimension cannot hold addresses
// people the register is never asked about: the task is created, nobody sees
// it, and nothing anywhere says why.
func TestAddressingAttributeMustFitItsDimension(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeMetadata(t, root, CatalogKind, roleCatalogID, `format: 1
id: `+roleCatalogID+`
name: РолиИсполнителей
title: {ru: Роли исполнителей}
code: {type: string, length: 9, auto: true}
description_length: 100
`)
	writeMetadata(t, root, InformationRegisterKind, performersID, `format: 1
id: `+performersID+`
name: ИсполнителиЗадач
title: {ru: Исполнители задач}
write_mode: independent
periodicity: none
dimensions:
  - id: b0000000-0000-4000-8000-000000000010
    name: РольИсполнителя
    title: {ru: Роль исполнителя}
    types: [{kind: catalog, reference: `+roleCatalogID+`}]
resources:
  - id: b0000000-0000-4000-8000-000000000011
    name: Исполнитель
    title: {ru: Исполнитель}
    types: [{kind: catalog, reference: `+roleCatalogID+`}]
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
  - id: b0000000-0000-4000-8000-000000000020
    name: РольИсполнителя
    title: {ru: Роль исполнителя}
    types: [{kind: string, length: 50}]
    dimension: b0000000-0000-4000-8000-000000000010
`)
	if _, err := Load(root); err == nil {
		t.Fatal("an addressing attribute its dimension cannot hold was accepted")
	}
}
