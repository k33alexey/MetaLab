package metadata

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
)

// nameAt is a name of the given number of letters.
func nameAt(length int) string {
	return strings.Repeat("Я", length)
}

// Every element named by an identifier inside an object - a point, a line and
// a decoration of a route, a predefined account and calculation type, a field
// of a source, a cube and a function, a template, method and handler of an
// HTTP service, an operation, procedure and parameter of a web service - takes
// a name of 255 letters, as the configurator does, and refuses 256 with the
// place named.
//
// Defect caught: a name of exactly 255 letters refused at any of the fourteen
// places.
func TestElementNamesTakeTheNameLimit(t *testing.T) {
	t.Parallel()
	configuration := metadataConfiguration()
	decodeProcess := func(text string) error {
		_, err := DecodeBusinessProcess("process.yaml", strings.NewReader(text), configuration)
		return err
	}
	route := func(point, line, decoration string) string {
		return `format: 1
id: ` + businessProcessID + `
name: Задание
title: {ru: Задание}
number: {type: string, length: 11, auto: true, unique: true, periodicity: none}
task: ` + taskID + `
route:
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: ` + point + `, kind: start}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {name: ` + line + `, from: ` + point + `, to: Завершение}
  decorations:
    - {name: ` + decoration + `, shape: Ellipse, location: {top: 0, left: 0, bottom: 10, right: 10}}
`
	}
	calculationTypes := func(name string) string {
		return `format: 1
id: ` + calculationTypesID + `
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 5}
description_length: 100
predefined:
  - {id: a0000000-0000-4000-8000-000000000010, name: ` + name + `, code: "00001"}
`
	}
	httpService := func(template, method, handler string) string {
		return `format: 1
id: ` + httpServiceID + `
name: Биллинг
title: {ru: Биллинг}
root_url: billing
templates:
  - id: f4000000-0000-4000-8000-000000000010
    name: ` + template + `
    title: {ru: Версия}
    template: /version
    methods:
      - {id: f4000000-0000-4000-8000-000000000011, name: ` + method + `, title: {ru: Получить}, method: GET, handler: ` + handler + `}
`
	}
	webService := func(operation, procedure, parameter string) string {
		return `format: 1
id: ` + webServiceID + `
name: ОбменДанными
title: {ru: Обмен данными}
namespace: http://example.org/exchange/1.0
operations:
  - id: f3000000-0000-4000-8000-000000000010
    name: ` + operation + `
    title: {ru: Проверка}
    procedure: ` + procedure + `
    return_type: {namespace: "http://www.w3.org/2001/XMLSchema", name: string}
    parameters:
      - id: f3000000-0000-4000-8000-000000000011
        name: ` + parameter + `
        title: {ru: Имя}
        type: {namespace: "http://www.w3.org/2001/XMLSchema", name: string}
        direction: in
`
	}
	for _, place := range []struct {
		what   string
		decode func(string) error
		text   func(name string) string
	}{
		{"route.points[0].name", decodeProcess, func(name string) string { return route(name, "Линия", "Фигура") }},
		{"route.transitions[0].name", decodeProcess, func(name string) string { return route("Старт", name, "Фигура") }},
		{"route.decorations[0].name", decodeProcess, func(name string) string { return route("Старт", "Линия", name) }},
		{"predefined[0].name", func(text string) error {
			_, err := DecodeChartOfAccounts("chart.yaml", strings.NewReader(text), configuration)
			return err
		}, func(name string) string {
			return strings.Replace(chartWithMask("@@@@@", 9, ""), "    name: Забалансовый", "    name: "+name, 1)
		}},
		{"predefined[0].name", func(text string) error {
			_, err := DecodeChartOfCalculationTypes("chart.yaml", strings.NewReader(text), configuration)
			return err
		}, calculationTypes},
		{"resources[0].name", func(text string) error {
			_, err := DecodeExternalCube("cube.yaml", strings.NewReader(text), configuration)
			return err
		}, func(name string) string {
			return "format: 1\nid: " + salesCube + "\nname: Продажи\ntitle: {ru: Продажи}\n" +
				strings.Replace(salesCubeBody, "    name: Количество", "    name: "+name, 1)
		}},
		{"fields[2].name", func(text string) error {
			_, err := DecodeExternalTable("table.yaml", strings.NewReader(text), configuration)
			return err
		}, func(name string) string {
			return "format: 1\nid: " + stockTable + "\nname: Остатки\ntitle: {ru: Остатки}\n" +
				strings.Replace(stockBody, "    name: Количество", "    name: "+name, 1)
		}},
		{"functions[0].name", func(text string) error {
			_, err := DecodeExternalDataSource("source.yaml", strings.NewReader(text), configuration)
			return err
		}, func(name string) string {
			return "format: 1\nid: " + warehouseSource + "\nname: Склад\ntitle: {ru: Склад}\nfunctions:\n  - id: " + firstFunction +
				"\n    name: " + name + "\n    title: {ru: Остаток}\n"
		}},
		{"templates[0].name", decodeHTTP(configuration), func(name string) string { return httpService(name, "Получить", "ВерсияПолучить") }},
		{"templates[0].methods[0].name", decodeHTTP(configuration), func(name string) string { return httpService("Версия", name, "ВерсияПолучить") }},
		{"templates[0].methods[0].handler", decodeHTTP(configuration), func(name string) string { return httpService("Версия", "Получить", name) }},
		{"operations[0].name", decodeWeb(configuration), func(name string) string { return webService(name, "Проверка", "Имя") }},
		{"operations[0].procedure", decodeWeb(configuration), func(name string) string { return webService("Проверка", name, "Имя") }},
		{"operations[0].parameters[0].name", decodeWeb(configuration), func(name string) string { return webService("Проверка", "Проверка", name) }},
	} {
		if err := place.decode(place.text(nameAt(maxNameLength))); err != nil {
			t.Errorf("%s of %d letters refused: %v", place.what, maxNameLength, err)
		}
		err := place.decode(place.text(nameAt(maxNameLength + 1)))
		if err == nil || !strings.Contains(err.Error(), place.what) {
			t.Errorf("%s of %d letters: %v", place.what, maxNameLength+1, err)
		}
	}
}

func decodeHTTP(configuration project.Project) func(string) error {
	return func(text string) error {
		_, err := DecodeHTTPService("service.yaml", strings.NewReader(text), configuration)
		return err
	}
}

func decodeWeb(configuration project.Project) func(string) error {
	return func(text string) error {
		_, err := DecodeWebService("service.yaml", strings.NewReader(text), configuration)
		return err
	}
}

// A line leaving a condition by the port of its own branch is the drawing the
// designer makes, and so is a line that gives no port, and a line giving a port
// out of a condition that names none: only a port of the other branch is
// refused, and it is - the neighbouring case is one line in the whole map.
//
// Defect caught: the port compared when the line gives none, so every line
// out of a condition with ports is refused; or not compared when it gives one.
func TestRouteConditionPortsAreComparedOnlyWhenTheLineGivesOne(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		ports, trueLine, falseLine string
		refused                    bool
	}{
		"обе линии без порта":              {ports: "true_port: 1, false_port: 3"},
		"обе линии через свой порт":        {ports: "true_port: 1, false_port: 3", trueLine: "from_port: 1", falseLine: "from_port: 3"},
		"порт линии у условия без портов":  {trueLine: "from_port: 2"},
		"ложная ветка через порт истинной": {ports: "true_port: 1, false_port: 3", falseLine: "from_port: 1", refused: true},
	} {
		root := routeOnlyProcess(t, `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000032, name: Проверка, kind: condition, `+test.ports+`}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Проверка}
    - {from: Проверка, to: Завершение, branch: "true", `+test.trueLine+`}
    - {from: Проверка, to: Завершение, branch: "false", `+test.falseLine+`}`)
		_, err := Load(root)
		if refused := err != nil && strings.Contains(err.Error(), "route.transitions[2] leaves the condition by the port of the other branch"); refused != test.refused || (err != nil && !refused) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A box of no width or no height is drawn, as a line is; only a box whose
// right edge is left of its left edge, or bottom above its top, is inside out.
// A decorative line attached to port 0 is attached; a negative port is none.
//
// Defect caught: a box of zero width or zero height refused, a line attached
// to port 0 refused.
func TestRouteBoxesOfNoSizeAndPortZeroAreDrawn(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		location, attached, want string
	}{
		"нулевая ширина":      {location: "{top: 0, left: 5, bottom: 10, right: 5}"},
		"нулевая высота":      {location: "{top: 5, left: 0, bottom: 5, right: 10}"},
		"вывернута по ширине": {location: "{top: 0, left: 6, bottom: 10, right: 5}", want: "route.points[0].location is drawn inside out"},
		"вывернута по высоте": {location: "{top: 6, left: 0, bottom: 5, right: 10}", want: "route.points[0].location is drawn inside out"},
		"привязка к порту 0":  {location: "{top: 0, left: 0, bottom: 10, right: 10}", attached: "0"},
		"привязка к порту −1": {location: "{top: 0, left: 0, bottom: 10, right: 10}", attached: "-1", want: "route.decorations[0].from.port must not be negative"},
	} {
		decorations := ""
		if test.attached != "" {
			decorations = "\n  decorations:\n    - {name: Указатель, line: [{x: 0, y: 0}, {x: 1, y: 1}], from: {item: Старт, port: " + test.attached + "}}"
		}
		root := routeOnlyProcess(t, `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start, location: `+test.location+`}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}`+decorations)
		_, err := Load(root)
		if test.want == "" && err != nil {
			t.Errorf("%s refused: %v", name, err)
		}
		if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
			t.Errorf("%s: %v, want one saying %q", name, err, test.want)
		}
	}
}

// The process handed out is a copy down to the kind of task it creates.
//
// Defect caught: the task shared with the catalog, so a caller changing it
// changes the configuration.
func TestBusinessProcessTaskIsHandedOutAsACopy(t *testing.T) {
	t.Parallel()
	root := routeOnlyProcess(t, `
  points:
    - {id: b0000000-0000-4000-8000-000000000030, name: Старт, kind: start}
    - {id: b0000000-0000-4000-8000-000000000034, name: Завершение, kind: completion}
  transitions:
    - {from: Старт, to: Завершение}`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	process, _ := catalog.BusinessProcess("Задание")
	if process.Task == nil || process.Task.String() != taskID {
		t.Fatalf("the process lost its task: %v", process.Task)
	}
	*process.Task = mustUUID(t, businessProcessID)
	again, _ := catalog.BusinessProcess("Задание")
	if again.Task.String() != taskID {
		t.Fatal("the task handed out is shared with the catalog")
	}
}

// A predefined account and a predefined calculation type take a description
// as long as the chart's description length and refuse one letter more; an
// account cannot be its own parent; the order may be as long as a varchar
// column holds, and with no order length it takes the width of the code.
//
// Defect caught: a description of exactly the declared length refused; the
// largest order length refused; an account naming itself as parent let
// through; an order of an account refused where the chart gives no order
// length.
func TestPredefinedOfChartsAtTheirEdges(t *testing.T) {
	t.Parallel()
	configuration := metadataConfiguration()
	accounts := func(text string) error {
		_, err := DecodeChartOfAccounts("chart.yaml", strings.NewReader(text), configuration)
		return err
	}
	calculation := func(text string) error {
		_, err := DecodeChartOfCalculationTypes("chart.yaml", strings.NewReader(text), configuration)
		return err
	}
	account := chartWithMask("@@@@@", 9, "")
	calculationType := `format: 1
id: ` + calculationTypesID + `
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 5}
description_length: 100
predefined:
  - {id: a0000000-0000-4000-8000-000000000010, name: Оклад, code: "00001", description: DESCRIPTION}
`
	for name, test := range map[string]struct {
		decode func(string) error
		text   string
		want   string
	}{
		"описание счёта во всю длину": {accounts, strings.Replace(account, "    description: Забалансовый", "    description: "+nameAt(120), 1), ""},
		"описание счёта длиннее": {accounts, strings.Replace(account, "    description: Забалансовый", "    description: "+nameAt(121), 1),
			"predefined[0].description must not exceed 120 characters"},
		"описание вида расчёта во всю длину": {calculation, strings.Replace(calculationType, "DESCRIPTION", nameAt(100), 1), ""},
		"описание вида расчёта длиннее": {calculation, strings.Replace(calculationType, "DESCRIPTION", nameAt(101), 1),
			"predefined[0].description must not exceed 100 characters"},
		"самая длинная длина порядка": {accounts, chartWithMask("@@@@@", maxVarcharLength, ""), ""},
		"длина порядка длиннее":       {accounts, chartWithMask("@@@@@", maxVarcharLength+1, ""), "order_length must be 0.."},
		"счёт сам себе родитель":      {accounts, account + "    parent: Забалансовый\n", "predefined[0] cannot be its own parent"},
		"порядок без длины порядка":   {accounts, chartWithMask("@@@@@", 0, "01"), ""},
	} {
		err := test.decode(test.text)
		if test.want == "" && err != nil {
			t.Errorf("%s refused: %v", name, err)
		}
		if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
			t.Errorf("%s: %v, want one saying %q", name, err, test.want)
		}
	}
}

// The rows of what displaces a calculation type and what leads it are keyed
// to the chart's own table; the rows of its base are not, since a base may
// come from several charts and one column cannot reference several tables.
//
// Defect caught: the key dropped from the displacing and leading rows, so a
// row may name a type that is not there; or put on the base rows, so a base
// from another chart cannot be written.
func TestCompetitionRowsAreKeyedToTheirChart(t *testing.T) {
	t.Parallel()
	definition, err := DecodeChartOfCalculationTypes("chart.yaml", strings.NewReader(`format: 1
id: `+calculationTypesID+`
name: Начисления
title: {ru: Начисления}
code: {type: string, length: 5}
description_length: 100
`), metadataConfiguration())
	if err != nil {
		t.Fatal(err)
	}
	_, parts, err := (&Catalog{}).chartOfCalculationTypesTables(definition)
	if err != nil {
		t.Fatal(err)
	}
	keyed := map[string]bool{}
	for _, part := range parts {
		for _, constraint := range part.Constraints {
			if strings.Contains(constraint.Definition, "FOREIGN KEY (calculation_type)") {
				keyed[part.Name] = true
			}
		}
	}
	id := mustUUID(t, calculationTypesID)
	for prefix, want := range map[string]bool{"tl": true, "tw": true, "tb": false} {
		if keyed[competitionTableName(prefix, id)] != want {
			t.Errorf("rows %s keyed to the chart: %v, want %v", prefix, keyed[competitionTableName(prefix, id)], want)
		}
	}
}

// A schedule counts days of the month to 31 and weeks of the month to 5 from
// either end, the ends included.
//
// Defect caught: the 31st, the last of the month (-31 from the end), the fifth
// week or the fifth from the end refused.
func TestJobScheduleCountsToTheEndsOfTheMonth(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		schedule JobSchedule
		want     string
	}{
		{JobSchedule{DayInMonth: 31}, ""},
		{JobSchedule{DayInMonth: -31}, ""},
		{JobSchedule{DayInMonth: -32}, "day_in_month must count at most 31"},
		{JobSchedule{WeekDayInMonth: 5}, ""},
		{JobSchedule{WeekDayInMonth: -5}, ""},
		{JobSchedule{WeekDayInMonth: 6}, "week_day_in_month must count at most five"},
	} {
		issues := strings.Join(validateJobSchedule("schedule", &test.schedule), "; ")
		if (test.want == "") != (issues == "") || !strings.Contains(issues, test.want) {
			t.Errorf("%+v: %q, want %q", test.schedule, issues, test.want)
		}
	}
}

// A transparent pixel at the very corner of the image is inside it.
//
// Defect caught: a pixel on the top row refused.
func TestTransparentPixelMayStandOnTheEdge(t *testing.T) {
	t.Parallel()
	for _, pixel := range []string{"{x: 0, y: 0}", "{x: 3, y: 0}"} {
		_, err := DecodeCommonPicture("object.yaml", strings.NewReader(`format: 1
id: `+commonPictureID+`
name: Печать
title: {ru: Печать}
load_transparent: true
transparent_pixel: `+pixel+`
`), metadataConfiguration())
		if err != nil {
			t.Errorf("pixel %s refused: %v", pixel, err)
		}
	}
}

// A file of a picture's folder is an image only with the extension of a format:
// a name with no extension, or ending in a bare dot, is neither an image nor a
// density.
//
// Defect caught: the empty extension taken for a format, so that any file
// without one in the folder of a picture is read as its image.
func TestPictureFileWantsAnExtension(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Печать", "Печать."} {
		if PictureFile(name) {
			t.Errorf("%q taken for an image", name)
		}
	}
	if !PictureFile("Печать.png") {
		t.Error("Печать.png not taken for an image")
	}
	for _, name := range []string{"100", "100."} {
		if _, ok := PictureDensity(name); ok {
			t.Errorf("%q taken for a density", name)
		}
	}
	if density, ok := PictureDensity("100.png"); !ok || density != 100 {
		t.Errorf("100.png read as %v, %v", density, ok)
	}
}

// A user of a client is one line wherever the control character stands, and
// a header name is a token of HTTP: DEL and a double quote are no more part of
// one than a space is.
//
// Defect caught: a control character at the very start of the user let
// through; DEL or a double quote let into a header name.
func TestWebSocketClientRefusesControlsAndSeparatorsAtEveryPlace(t *testing.T) {
	t.Parallel()
	if message := refusedWebSocketClient(t, "user: \"\\x01robot\"\n", "a user starting with a control"); !strings.Contains(message, "user must be one line") {
		t.Fatalf("the error does not say what is wrong: %v", message)
	}
	for _, name := range []string{"X-Tenant\x7f", "\"X-Tenant", "X-Tenant\""} {
		if httpToken(name) {
			t.Errorf("%q taken for a header name", name)
		}
	}
	if !httpToken("X-Tenant") {
		t.Error("X-Tenant refused as a header name")
	}
}

// Parameters of an expression are read whole, and reading goes on right after
// each: &10 is the tenth, not the first and a zero; &9 is a parameter; &2 right
// after &1[] is seen, and so is &2 right after an ampersand that is not one.
//
// Defect caught: a parameter with the digit 0 or 9 misread, or the character
// after a parameter or after a lone ampersand skipped - each lets a variable
// parameter that is not the last through.
func TestExpressionParametersAreReadWhole(t *testing.T) {
	t.Parallel()
	for expression, says := range map[string]string{
		"f(&1[], &10)": "the last here is &10",
		"f(&1[], &9)":  "the last here is &9",
		"f(&1[]&2)":    "the last here is &2",
		"f(&1[], &&2)": "the last here is &2",
	} {
		issues := strings.Join(validateExpressionParameters("expression", expression), "; ")
		if !strings.Contains(issues, says) {
			t.Errorf("%s: %q, want one saying %q", expression, issues, says)
		}
	}
	if issues := validateExpressionParameters("expression", "f(&1, &10[])"); len(issues) != 0 {
		t.Errorf("a variable tenth parameter, the last, refused: %v", issues)
	}
}

// An event is spelled in kebab case at every capital, the first and last
// letters of the alphabet included.
//
// Defect caught: the A of AfterWrite or a Z left as it is.
func TestSubscriptionEventSpellingTakesEveryCapital(t *testing.T) {
	t.Parallel()
	for event, want := range map[string]string{
		"AfterWriteDataHistoryVersionsProcessing": "after-write-data-history-versions-processing",
		"OnAutoCreateNewNode":                     "on-auto-create-new-node",
		"OnZ":                                     "on-z",
	} {
		if got := eventSubscriptionName(event); got != want {
			t.Errorf("%s spelled %q, want %q", event, got, want)
		}
	}
}

// An automatic identifier as long as its column is written padded to nothing;
// one digit more is refused.
//
// Defect caught: an identifier of exactly the declared length refused, so the
// last number of the sequence cannot be given out.
func TestAutomaticIdentifierFillsItsWholeLength(t *testing.T) {
	t.Parallel()
	if got, err := formatAutomaticIdentifier("999", StringType, 3); err != nil || got != "999" {
		t.Fatalf("an identifier of the whole length: %q, %v", got, err)
	}
	if got, err := formatAutomaticIdentifier("7", StringType, 3); err != nil || got != "007" {
		t.Fatalf("a short identifier: %q, %v", got, err)
	}
	if _, err := formatAutomaticIdentifier("1000", StringType, 3); err == nil {
		t.Fatal("an identifier longer than its column accepted")
	}
}

// A cube, a web service and a task handed out are copies down to what lies
// behind a pointer or in a list: the resources of a cube, the answer type of an
// operation, the register dimension an addressing attribute is matched with.
//
// Defect caught: any of the three shared with the catalog, so a caller
// changing its copy changes the configuration.
func TestKindsAreHandedOutAsCopiesBehindPointers(t *testing.T) {
	t.Parallel()
	cube := ExternalCube{Resources: []ExternalCubeResource{{Attribute: Attribute{Name: "Количество"}}}}
	cloneExternalCube(cube).Resources[0].Name = "Испорчено"
	if cube.Resources[0].Name != "Количество" {
		t.Error("the resources of a cube are shared with the copy")
	}

	service := WebServiceDefinition{Operations: []WebServiceOperation{{ReturnType: &XMLTypeName{Name: "string"}}}}
	cloneWebService(service).Operations[0].ReturnType.Name = "boolean"
	if service.Operations[0].ReturnType.Name != "string" {
		t.Error("the answer type of an operation is shared with the copy")
	}

	dimension := mustUUID(t, "b0000000-0000-4000-8000-000000000010")
	task := TaskDefinition{AddressingAttributes: []AddressingAttribute{{Dimension: &dimension}}}
	*cloneTask(task).AddressingAttributes[0].Dimension = mustUUID(t, taskID)
	if *task.AddressingAttributes[0].Dimension != mustUUID(t, "b0000000-0000-4000-8000-000000000010") {
		t.Error("the dimension of an addressing attribute is shared with the copy")
	}
}

// A task is searched in its list by description only when it has one.
//
// Defect caught: an index built on a description column a task without a
// description does not have, which the database refuses; or none built where
// there is a description.
func TestTaskIsSearchedByDescriptionOnlyWhenItHasOne(t *testing.T) {
	t.Parallel()
	for length, want := range map[int]bool{0: false, 150: true} {
		definition, err := DecodeTask("task.yaml", strings.NewReader(fmt.Sprintf(`format: 1
id: %s
name: ЗадачаИсполнителя
title: {ru: Задача исполнителя}
number: {type: string, length: 14, auto: true, unique: true, periodicity: none}
description_length: %d
`, taskID, length)), metadataConfiguration())
		if err != nil {
			t.Fatal(err)
		}
		table, _, err := (&Catalog{}).taskTables(definition)
		if err != nil {
			t.Fatal(err)
		}
		searched := false
		for _, index := range table.Indexes {
			if strings.Contains(strings.Join(index.Keys, ","), "description") {
				searched = true
			}
		}
		if searched != want {
			t.Errorf("description length %d: searched by description %v, want %v", length, searched, want)
		}
	}
}

// contactCharacteristic is a characteristic of the goods read from the catalog
// of contact kinds, with the field of that catalog its data path names, and
// with the values kept by the given object.
func contactCharacteristic(dataPath, valuesObject string) string {
	return `characteristics:
  - types:
      table: {kind: catalogs, object: ` + characteristicContactKinds + `}
      key: {standard: ref}
      data_path: ` + dataPath + `
    values:
      table: {kind: catalogs, object: ` + characteristicGoods + `, table_part: ` + characteristicGoodsPart + `}
      object: ` + valuesObject + `
      type: {attribute: ` + characteristicGoodsContact + `}
      value: {attribute: ` + characteristicGoodsValue + `}
`
}

// A characteristic reads the code and the description of its table of kinds
// only where that table has them, and a field it names in passing - the data
// path - is checked as the key is, without the rest of the description being
// left unchecked.
//
// Defect caught: a code or a description taken as there on a catalog without
// one, or as missing on a catalog with one; a data path naming a field that is
// not there let through; a good data path ending the check before the table of
// values is looked at.
func TestCharacteristicReadsTheStandardFieldsTheTableHas(t *testing.T) {
	t.Parallel()
	contactKinds := func(code string, description int) string {
		return fmt.Sprintf(`format: 1
id: %s
name: ВидыКонтактнойИнформации
title: {ru: Виды контактной информации}
%sdescription_length: %d
hierarchy: {enabled: true, kind: folders-and-items}
`, characteristicContactKinds, code, description)
	}
	const withCode, without = "code: {type: string, length: 9, auto: true}\n", "code: {type: string, length: 0}\n"
	for name, test := range map[string]struct {
		kinds, dataPath, valuesObject, want string
	}{
		"код у справочника с кодом":                {contactKinds(withCode, 150), "{standard: code}", "{standard: ref}", ""},
		"код у справочника без кода":               {contactKinds(without, 150), "{standard: code}", "{standard: ref}", "types.data_path names the standard field code"},
		"наименование у справочника с ним":         {contactKinds(withCode, 150), "{standard: description}", "{standard: ref}", ""},
		"наименование у справочника без него":      {contactKinds(withCode, 0), "{standard: description}", "{standard: ref}", "types.data_path names the standard field description"},
		"верный путь, значения не про этот объект": {contactKinds(withCode, 150), "{standard: ref}", "{attribute: " + characteristicGoodsContact + "}", "keeps no values of it"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := characteristicsProject(t)
			writeMetadata(t, root, CatalogKind, characteristicContactKinds, test.kinds)
			writeMetadata(t, root, CatalogKind, characteristicGoods, goodsWithCharacteristics(contactCharacteristic(test.dataPath, test.valuesObject)))
			_, err := Load(root)
			if test.want == "" && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("%v, want one saying %q", err, test.want)
			}
		})
	}
}

// A table of a source has a parent to read only when it nests.
//
// Defect caught: the parent of a flat table taken as there, or of a nesting
// one as missing.
func TestCharacteristicReadsTheParentOfANestingTableOfASource(t *testing.T) {
	t.Parallel()
	const goodsRef = "{kind: external-data-source-table, reference: " + goodsTable + "}"
	const parentField = "f8000000-0000-4000-8000-000000000087"
	nesting := `name_in_data_source: dbo.PropertyKinds
data_type: object
key_fields: [Код]
hierarchy:
  parent_field: Родитель
fields:
  - id: ` + propertyKindsKey + `
    name: Код
    title: {ru: Код}
    types: [{kind: string, length: 10}]
    name_in_data_source: code
  - id: ` + parentField + `
    name: Родитель
    title: {ru: Родитель}
    types: [{kind: external-data-source-table, reference: ` + propertyKindsTable + `}]
    name_in_data_source: parent
`
	characteristics := strings.Replace(goodsCharacteristics(propertyKindsTable, "{standard: ref}"),
		"      key: {standard: ref}\n", "      key: {standard: ref}\n      data_path: {standard: parent}\n", 1)
	for nests, want := range map[bool]string{true: "", false: "types.data_path names the standard field parent"} {
		root := metadataProject(t)
		writeExternalSource(t, root, warehouseSource, "Склад", "")
		writeExternalTable(t, root, "Склад", goodsTable, "Товары", goodsBody+characteristics)
		writePropertyTables(t, root, goodsRef)
		if nests {
			writeExternalTable(t, root, "Склад", propertyKindsTable, "ВидыСвойств", nesting)
		}
		_, err := Load(root)
		if want == "" && err != nil {
			t.Errorf("the parent of a nesting table refused: %v", err)
		}
		if want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("the parent of a flat table: %v, want one saying %q", err, want)
		}
	}
}
