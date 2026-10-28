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
// group of the navigation panel, and not one in a group of a form. Defects
// caught: a command in the wrong group, seen or unseen wrongly, titled
// wrongly, given where the kind or the object gives none.
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
	catalog := &Catalog{Catalogs: []CatalogDefinition{currencies}, Documents: []DocumentDefinition{order, quiet},
		InformationRegisters: []InformationRegisterDefinition{prices, rates}, AccumulationRegisters: []AccumulationRegisterDefinition{stock},
		Enumerations: []Enumeration{kindsOf}, Reports: []ReportDefinition{profit}, Constants: []Constant{currency},
		CommandGroups: []CommandGroupDefinition{sales, form}, CommonCommands: []CommonCommandDefinition{exchange, printing},
		commandGroupByID: map[uuid.UUID]int{sales.ID: 0, form.ID: 1}}
	got := catalog.SectionLines([]uuid.UUID{id(1), id(2), id(3), id(4), id(5), id(6), id(7), id(8), id(9), id(12), id(13)})
	type line struct {
		Object, Command, Standard, Group, Kind, Title string
		Create, Visible                               bool
	}
	want := []line{
		{intUUID(1), "", "open-list", "navigation-panel-ordinary", "catalogs", "Валюты", false, true},
		{intUUID(1), "", "create", "actions-panel-create", "catalogs", "Валюта", true, false},
		{intUUID(2), "", "open-list", "navigation-panel-ordinary", "documents", "Список заказов", false, true},
		{intUUID(2), "", "create", "actions-panel-create", "documents", "Заказ клиента", true, false},
		{"", intUUID(21), "", "navigation-panel-important", "documents", "Заказы", false, true},
		{intUUID(4), "", "open-list", "navigation-panel-ordinary", "information-registers", "Цены", false, true},
		{intUUID(4), "", "create", "actions-panel-create", "information-registers", "Цены", true, false},
		{intUUID(5), "", "open-list", "navigation-panel-ordinary", "information-registers", "Курсы", false, false},
		{intUUID(6), "", "open-list", "navigation-panel-ordinary", "accumulation-registers", "Остатки", false, false},
		{intUUID(7), "", "open-list", "navigation-panel-ordinary", "enumerations", "Виды", false, false},
		{intUUID(8), "", "open", "actions-panel-reports", "reports", "Прибыль", false, true},
		{intUUID(9), "", "open", "actions-panel-tools", "constants", "Основная валюта", false, true},
		{"", intUUID(12), "", "", "common-commands", "Обмен", false, true},
	}
	var read []line
	for _, item := range got {
		value := line{Standard: string(item.Standard), Group: string(item.Group.Standard), Kind: string(item.Kind), Title: item.Title["ru"],
			Create: item.Create, Visible: item.Visible}
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
