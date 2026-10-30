package metadata

import (
	"encoding/json"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// The commands the content of a subsystem gives its section are the
// prototype's (docs/block-00/podsistema.md, «Командный интерфейс…»,
// «Досъёмка 28.10.2026»), written out here kind by kind, not read from the
// descriptor: a catalog's list in «Обычное», visible, by its synonym when it
// has no presentation of a list, and its creation in «Создать», unseen, by
// the presentation of an object; a document by its presentations, and its
// own command in the group of its property, visible, while one in a group
// of a form is not the section's; a document of no standard commands gives
// none; an independent register of information its list visible and its
// creation, one a recorder writes its list unseen; a register of
// accumulation and an enumeration their list unseen; a report in «Отчеты»
// and a constant in «Сервис», visible; a common command in a project's
// group of the navigation panel, and not one in a group of a form; a
// catalog of folders and items also the creation of a folder in «Создать»,
// unseen, and one of items alone none (podsistema.md, «Досъёмка
// 28.10.2026, вечер»), titled by the presentation of the list or else the
// synonym - not by the presentation of an object, as «: создать» is - and
// so a chart of characteristic types with a hierarchy, one without none
// («Досъёмка 28.10.2026, поздний вечер»; vopr-2.636/23, 24); a common form of standard
// commands its opening in «Сервис», visible, by its synonym, and one
// without none. Defects caught: a command in the wrong group, seen or
// unseen wrongly, titled wrongly, given where the kind or the object gives
// none.
func TestTheContentOfASubsystemGivesItsSectionThePrototypesCommands(t *testing.T) {
	t.Parallel()
	id := func(number int) uuid.UUID { value, _ := uuid.Parse(numbered(number)); return value }
	ru := func(text string) LocalizedText { return LocalizedText{"ru": text} }
	currencies := CatalogDefinition{ID: id(1), Name: "Валюты", Title: ru("Валюты")}
	currencies.UseStandardCommands, currencies.ObjectPresentation = true, ru("Валюта")
	order := DocumentDefinition{ID: id(2), Name: "ЗаказКлиента", Title: ru("Заказ клиента"),
		Commands: []ObjectCommand{{ID: id(21), Name: "Заказы", Title: ru("Заказы"), Group: "navigation-panel-important"},
			{ID: id(22), Name: "Печать", Title: ru("Печать"), Group: "form-command-bar-important"}}}
	order.UseStandardCommands, order.ListPresentation, order.ObjectPresentation = true, ru("Список заказов"), ru("Заказ клиента")
	quiet := DocumentDefinition{ID: id(3), Name: "Тихий", Title: ru("Тихий")}
	prices := InformationRegisterDefinition{ID: id(4), Name: "Цены", Title: ru("Цены"), WriteMode: InformationRegisterIndependent}
	prices.UseStandardCommands = true
	rates := InformationRegisterDefinition{ID: id(5), Name: "Курсы", Title: ru("Курсы"), WriteMode: InformationRegisterRecorder}
	rates.UseStandardCommands = true
	stock := AccumulationRegisterDefinition{ID: id(6), Name: "Остатки", Title: ru("Остатки")}
	stock.UseStandardCommands = true
	kindsOf := Enumeration{ID: id(7), Name: "Виды", Title: ru("Виды")}
	kindsOf.UseStandardCommands = true
	profit := ReportDefinition{ID: id(8), Name: "Прибыль", Title: ru("Прибыль")}
	profit.UseStandardCommands = true
	currency := Constant{ID: id(9), Name: "Валюта", Title: ru("Основная валюта"), UseStandardCommands: true}
	sales := CommandGroupDefinition{ID: id(10), Name: "Продажи", Title: ru("Продажи"), Category: NavigationPanelCategory}
	form := CommandGroupDefinition{ID: id(11), Name: "Формы", Title: ru("Формы"), Category: FormCommandBarCategory}
	salesRef, formRef := sales.ID, form.ID
	exchange := CommonCommandDefinition{ObjectCommand: ObjectCommand{ID: id(12), Name: "Обмен", Title: ru("Обмен"), GroupRef: &salesRef}}
	printing := CommonCommandDefinition{ObjectCommand: ObjectCommand{ID: id(13), Name: "Печать", Title: ru("Печать"), GroupRef: &formRef}}
	crews := CatalogDefinition{ID: id(14), Name: "Бригады", Title: ru("Бригады"), Hierarchy: Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy}}
	crews.UseStandardCommands, crews.ObjectPresentation, crews.ListPresentation = true, ru("Бригада"), ru("Список бригад")
	goods := CatalogDefinition{ID: id(18), Name: "Номенклатура", Title: ru("Номенклатура"), Hierarchy: Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy}}
	goods.UseStandardCommands, goods.ObjectPresentation = true, ru("Товар")
	properties := ChartOfCharacteristicTypesDefinition{ID: id(19), Name: "Свойства", Title: ru("Свойства"),
		Hierarchy: Hierarchy{Enabled: true, Kind: FoldersAndItemsHierarchy}}
	properties.UseStandardCommands = true
	flat := ChartOfCharacteristicTypesDefinition{ID: id(20), Name: "Плоские", Title: ru("Плоские")}
	flat.UseStandardCommands = true
	staff := CatalogDefinition{ID: id(15), Name: "Сотрудники", Title: ru("Сотрудники"), Hierarchy: Hierarchy{Enabled: true, Kind: ItemsHierarchy}}
	staff.UseStandardCommands = true
	catalog := &Catalog{Catalogs: []CatalogDefinition{currencies, crews, staff, goods},
		InformationRegisters: []InformationRegisterDefinition{prices, rates}, AccumulationRegisters: []AccumulationRegisterDefinition{stock},
		Enumerations: []Enumeration{kindsOf}, Reports: []ReportDefinition{profit}, Constants: []Constant{currency},
		Documents: []DocumentDefinition{order, quiet}, ChartsOfCharacteristicTypes: []ChartOfCharacteristicTypesDefinition{properties, flat},
		CommandGroups: []CommandGroupDefinition{sales, form}, CommonCommands: []CommonCommandDefinition{exchange, printing},
		commandGroupByID: map[uuid.UUID]int{sales.ID: 0, form.ID: 1},
		commonFormHeads: []commonFormHead{{ID: id(16), Name: "АдреснаяКнига", Title: ru("Адресная книга"), UseStandardCommands: true},
			{ID: id(17), Name: "АвтономнаяРабота", Title: ru("Автономная работа")}}}
	got := catalog.SectionLines([]uuid.UUID{id(1), id(2), id(3), id(4), id(5), id(6), id(7), id(8), id(9), id(12), id(13), id(14), id(15), id(16), id(17), id(18), id(19), id(20)})
	type line struct {
		Object, Command, Standard, Group, Kind, Title string
		Visible                                       bool
	}
	want := []line{
		{intUUID(1), "", "open-list", "navigation-panel-ordinary", "catalogs", "Валюты", true},
		{intUUID(1), "", "create", "actions-panel-create", "catalogs", "Валюта", false},
		{intUUID(2), "", "open-list", "navigation-panel-ordinary", "documents", "Список заказов", true},
		{intUUID(2), "", "create", "actions-panel-create", "documents", "Заказ клиента", false},
		{"", intUUID(21), "", "navigation-panel-important", "documents", "Заказы", true},
		{intUUID(4), "", "open-list", "navigation-panel-ordinary", "information-registers", "Цены", true},
		{intUUID(4), "", "create", "actions-panel-create", "information-registers", "Цены", false},
		{intUUID(5), "", "open-list", "navigation-panel-ordinary", "information-registers", "Курсы", false},
		{intUUID(6), "", "open-list", "navigation-panel-ordinary", "accumulation-registers", "Остатки", false},
		{intUUID(7), "", "open-list", "navigation-panel-ordinary", "enumerations", "Виды", false},
		{intUUID(8), "", "open", "actions-panel-reports", "reports", "Прибыль", true},
		{intUUID(9), "", "open", "actions-panel-tools", "constants", "Основная валюта", true},
		{"", intUUID(12), "", "", "common-commands", "Обмен", true},
		{intUUID(14), "", "open-list", "navigation-panel-ordinary", "catalogs", "Список бригад", true},
		{intUUID(14), "", "create", "actions-panel-create", "catalogs", "Бригада", false},
		{intUUID(14), "", "create-folder", "actions-panel-create", "catalogs", "Список бригад", false},
		{intUUID(15), "", "open-list", "navigation-panel-ordinary", "catalogs", "Сотрудники", true},
		{intUUID(15), "", "create", "actions-panel-create", "catalogs", "Сотрудники", false},
		{intUUID(16), "", "open", "actions-panel-tools", "common-forms", "Адресная книга", true},
		{intUUID(18), "", "open-list", "navigation-panel-ordinary", "catalogs", "Номенклатура", true},
		{intUUID(18), "", "create", "actions-panel-create", "catalogs", "Товар", false},
		{intUUID(18), "", "create-folder", "actions-panel-create", "catalogs", "Номенклатура", false},
		{intUUID(19), "", "open-list", "navigation-panel-ordinary", "charts-of-characteristic-types", "Свойства", true},
		{intUUID(19), "", "create", "actions-panel-create", "charts-of-characteristic-types", "Свойства", false},
		{intUUID(19), "", "create-folder", "actions-panel-create", "charts-of-characteristic-types", "Свойства", false},
		{intUUID(20), "", "open-list", "navigation-panel-ordinary", "charts-of-characteristic-types", "Плоские", true},
		{intUUID(20), "", "create", "actions-panel-create", "charts-of-characteristic-types", "Плоские", false},
	}
	var read []line
	for _, item := range got {
		value := line{Standard: string(item.Standard), Group: string(item.Group.Standard), Kind: string(item.Kind), Title: item.Title["ru"],
			Visible: item.Visible}
		if item.Object != nil {
			value.Object = item.Object.String()
		}
		if item.Command != nil {
			value.Command = item.Command.String()
		}
		if item.Group.Group != nil && *item.Group.Group != sales.ID {
			value.Group = "a group not of the section"
		}
		read = append(read, value)
	}
	gotJSON, _ := json.Marshal(read)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("lines:\n got %s\nwant %s", gotJSON, wantJSON)
	}
	// An object's own command knows its object, which a role is asked about
	// too (TestARoleOpensACommandOfASectionByThePrototypesRight); a common
	// command and a standard one have none.
	for _, item := range got {
		owned := item.Command != nil && *item.Command == id(21)
		if owned != (item.Owner != nil) || owned && *item.Owner != id(2) {
			t.Errorf("the line %+v has the owner %v", item.InterfaceCommand, item.Owner)
		}
	}
}

// intUUID is an identifier numbered for a test, as text.
func intUUID(number int) string { return numbered(number) }

func numbered(number int) string {
	digits := []byte("000000000000")
	for at := len(digits) - 1; number > 0 && at >= 0; at-- {
		digits[at] = byte('0' + number%10)
		number /= 10
	}
	return "00000000-0000-4000-8000-" + string(digits)
}

// A role opens a command of a section by the right the prototype asks
// (obshchee.md, «Видимость по ролям»; snapshots vopr-2.636/02-06,
// dosyomka-2639/06-11), written out here, not read from the code: a list by
// «Просмотр» of its object, a creation by «Интерактивное добавление»; a
// constant and a common command by «Просмотр» of themselves, «Чтение» of a
// constant opening nothing; an object's own command by «Просмотр» of the
// command and of its object both - a role with «Просмотр» of the command
// alone opens nothing, nor one with «Просмотр» of the object alone. Defects
// caught: an own command opened by the right on the command alone, as
// before; a right of another object or another command taken; the owner
// asked of a common command.
func TestARoleOpensACommandOfASectionByThePrototypesRight(t *testing.T) {
	t.Parallel()
	id := func(number int) uuid.UUID { value, _ := uuid.Parse(numbered(number)); return value }
	order, own, other, constant, common := id(1), id(2), id(3), id(4), id(5)
	view, read, insert := []PermissionOperation{PermissionView}, []PermissionOperation{PermissionRead}, []PermissionOperation{PermissionInteractiveInsert}
	line := func(object *uuid.UUID, command *uuid.UUID, standard StandardCommand, owner *uuid.UUID) SectionLine {
		return SectionLine{InterfaceCommand: InterfaceCommand{Object: object, Command: command, Standard: standard}, Owner: owner}
	}
	list, create := line(&order, nil, StandardOpenList, nil), line(&order, nil, StandardCreate, nil)
	ownCommand, constantLine, commonLine := line(nil, &own, "", &order), line(&constant, nil, StandardOpen, nil), line(nil, &common, "", nil)
	role := func(objects ...ObjectPermission) RoleDefinition { return RoleDefinition{Objects: objects} }
	for name, testCase := range map[string]struct {
		role  RoleDefinition
		line  SectionLine
		opens bool
	}{
		"the list by «Просмотр»":                     {role(ObjectPermission{Object: order, Operations: view}), list, true},
		"the list not by «Чтение»":                   {role(ObjectPermission{Object: order, Operations: read}), list, false},
		"the creation by «Интерактивное добавление»": {role(ObjectPermission{Object: order, Operations: insert}), create, true},
		"the creation not by «Просмотр»":             {role(ObjectPermission{Object: order, Operations: view}), create, false},
		"a constant by «Просмотр»":                   {role(ObjectPermission{Object: constant, Operations: view}), constantLine, true},
		"a constant not by «Чтение»":                 {role(ObjectPermission{Object: constant, Operations: read}), constantLine, false},
		"a common command by «Просмотр»":             {role(ObjectPermission{Object: common, Operations: view}), commonLine, true},
		"an own command by both":                     {role(ObjectPermission{Object: order, Operations: view, Fields: []FieldPermission{{Field: own.String(), Operations: view}}}), ownCommand, true},
		"an own command not by itself alone":         {role(ObjectPermission{Object: order, Fields: []FieldPermission{{Field: own.String(), Operations: view}}}), ownCommand, false},
		"an own command not by the object alone":     {role(ObjectPermission{Object: order, Operations: view}), ownCommand, false},
		"an own command not by another command":      {role(ObjectPermission{Object: order, Operations: view, Fields: []FieldPermission{{Field: other.String(), Operations: view}}}), ownCommand, false},
		"nothing written, nothing opened":            {role(), list, false},
	} {
		if got := testCase.role.OpensSectionLine(testCase.line); got != testCase.opens {
			t.Errorf("%s: opens %v, want %v", name, got, testCase.opens)
		}
	}
}
