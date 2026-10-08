package studio

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// bigManagedForm builds a form the size of the largest the configurations
// being moved hold - the common form ГС_ИнтерфейсСпециалистаЛО of lombard1:
// about 1400 elements, each with its extended tooltip and the fields and
// tables with their context menus, 802 columns in tables, 190 commands and
// 247 attributes. The form itself is not ours and does not go into the
// repository; its shape does.
func bigManagedForm(name string) metadata.ManagedForm {
	serial := 0
	element := func(kind metadata.FormElementKind, prefix string) metadata.ManagedFormElement {
		serial++
		item := metadata.ManagedFormElement{ID: uuid.MustNew(), Name: fmt.Sprintf("%s%d", prefix, serial), Kind: kind,
			Title: metadata.LocalizedText{"ru": fmt.Sprintf("Элемент %d", serial)}}
		item.ExtendedTooltip = &metadata.ManagedFormElement{ID: uuid.MustNew(), Name: item.Name + "РасширеннаяПодсказка",
			Kind: metadata.FormElementLabelDecoration}
		if kind.IsField() || kind == metadata.FormElementTable {
			item.ContextMenu = &metadata.FormAttachedMenu{ID: uuid.MustNew(), Name: item.Name + "КонтекстноеМеню"}
		}
		return item
	}
	form := metadata.ManagedForm{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: name, Title: metadata.LocalizedText{"ru": "Рабочее место"},
		Kind: metadata.CommonForm}
	for index := range 190 {
		command := fmt.Sprintf("Команда%d", index)
		form.Commands = append(form.Commands, metadata.ManagedFormCommand{ID: uuid.MustNew(), Name: command,
			Title: metadata.LocalizedText{"ru": command}, Action: metadata.FormCommandCustom, Handler: command})
	}
	for index := range 247 {
		attribute := fmt.Sprintf("Реквизит%d", index)
		form.Attributes = append(form.Attributes, metadata.FormAttribute{ID: uuid.MustNew(), Name: attribute,
			Types: []metadata.Type{{Kind: metadata.StringType, Length: 100}}})
	}
	pages := element(metadata.FormElementPages, "Страницы")
	for page := range 55 {
		current := element(metadata.FormElementPage, "Страница")
		for range 2 {
			group := element(metadata.FormElementUsualGroup, "Группа")
			for range 5 {
				group.Children = append(group.Children, element(metadata.FormElementInputField, "Поле"))
				group.Children = append(group.Children, element(metadata.FormElementLabelField, "Надпись"))
			}
			current.Children = append(current.Children, group)
		}
		buttons := element(metadata.FormElementButtonGroup, "Кнопки")
		for button := range 6 {
			item := element(metadata.FormElementButton, "Кнопка")
			item.Command = "Form.Command." + form.Commands[(page*6+button)%len(form.Commands)].Name
			buttons.Children = append(buttons.Children, item)
		}
		current.Children = append(current.Children, buttons)
		if page < 10 {
			table := element(metadata.FormElementTable, "Таблица")
			for range 10 {
				columns := element(metadata.FormElementColumnGroup, "Колонки")
				for range 8 {
					columns.Children = append(columns.Children, element(metadata.FormElementLabelField, "Колонка"))
				}
				table.Children = append(table.Children, columns)
			}
			current.Children = append(current.Children, table)
		}
		pages.Children = append(pages.Children, current)
	}
	form.Items = []metadata.ManagedFormElement{pages}
	return form
}

// countElements counts the elements of a form with what they hold.
func countElements(items []metadata.ManagedFormElement) int {
	count := 0
	for _, item := range items {
		count += 1 + countElements(item.Nested())
	}
	return count
}

// writeBigFormProject writes a project holding the big form as a common
// form, and as many catalogs beside it as a configuration of the size of erp
// has of its own objects to read.
func writeBigFormProject(t testing.TB, catalogs int) (*Workspace, string) {
	t.Helper()
	root := createProject(t)
	// The form is the form of an object of a catalog, so that opening it
	// goes the whole way: the designer is offered the data of its object.
	form := bigManagedForm("РабочееМесто")
	form.Kind = metadata.ObjectForm
	relative, err := project.ObjectFormPath("catalogs", "Залоги", form.Name)
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	if err := metadata.Encode(&source, form); err != nil {
		t.Fatal(err)
	}
	writeBigFormFile(t, root, relative, source.Bytes())
	owner := metadata.CatalogDefinition{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: "Залоги", Title: metadata.LocalizedText{"ru": "Залоги"},
		Code: metadata.CatalogCode{Type: metadata.StringType, Length: 9}, DescriptionLength: 50,
		Attributes: []metadata.Attribute{{ID: uuid.MustNew(), Name: "Оценка", Title: metadata.LocalizedText{"ru": "Оценка"},
			Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}}}},
		Forms: metadata.HierarchicalObjectForms{ObjectForms: metadata.ObjectForms{Object: form.Name}}}
	ownerPath, err := project.ObjectMetadataPath("catalogs", owner.Name)
	if err != nil {
		t.Fatal(err)
	}
	source.Reset()
	if err := metadata.Encode(&source, owner); err != nil {
		t.Fatal(err)
	}
	writeBigFormFile(t, root, ownerPath, source.Bytes())
	for index := range catalogs {
		catalog := metadata.CatalogDefinition{Format: metadata.CurrentFormat, ID: uuid.MustNew(), Name: fmt.Sprintf("Справочник%d", index),
			Title: metadata.LocalizedText{"ru": "Справочник"}, Code: metadata.CatalogCode{Type: metadata.StringType, Length: 9},
			DescriptionLength: 50}
		for attribute := range 10 {
			catalog.Attributes = append(catalog.Attributes, metadata.Attribute{ID: uuid.MustNew(), Name: fmt.Sprintf("Реквизит%d", attribute),
				Title: metadata.LocalizedText{"ru": "Реквизит"}, Types: []metadata.Type{{Kind: metadata.StringType, Length: 50}}})
		}
		path, err := project.ObjectMetadataPath("catalogs", catalog.Name)
		if err != nil {
			t.Fatal(err)
		}
		source.Reset()
		if err := metadata.Encode(&source, catalog); err != nil {
			t.Fatal(err)
		}
		writeBigFormFile(t, root, path, source.Bytes())
	}
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return workspace, relative
}

func writeBigFormFile(t testing.TB, root, relative string, content []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// responsivenessBudget is a budget of ML-STUDIO.md, «Отзывчивость», set for
// the developer's machine. A slower machine that runs the checks - the
// runner of GitHub - says how much slower it is in ML_BUDGET_SCALE, so that
// the number itself is never loosened.
//
// A budget is measured on a machine doing nothing else: run beside the
// other packages of go test ./..., the opening of the largest form took
// 270-410 ms against 75 ms alone. So it runs only where it is asked for,
// make test-budgets, which sets ML_BUDGETS and runs it alone.
func responsivenessBudget(t *testing.T, budget time.Duration) time.Duration {
	t.Helper()
	if os.Getenv("ML_BUDGETS") == "" {
		t.Skip("a budget is measured alone, by make test-budgets")
	}
	if raceEnabled {
		t.Skip("the race detector slows the code fifteen times; the budgets are checked by make test-budgets")
	}
	if scale, err := strconv.ParseFloat(os.Getenv("ML_BUDGET_SCALE"), 64); err == nil && scale >= 1 {
		budget = time.Duration(float64(budget) * scale)
	}
	return budget
}

// The largest form of the configurations being moved opens in the designer
// within the budget of opening a form, 300 ms, however large the project
// around it: the median of five openings through the Studio's own request,
// from reading the file to the JSON the browser receives.
//
// Defect caught: opening a form loading the whole project to find the
// object it belongs to - a second for every two thousand objects beside it,
// five seconds on a project the size of erp (2.242); a form of 5 thousand
// elements, tooltips and menus read or checked so slowly that it alone
// misses the budget.
func TestOpeningTheLargestFormStaysWithinItsBudget(t *testing.T) {
	budget := responsivenessBudget(t, 300*time.Millisecond)
	workspace, relative := writeBigFormProject(t, 2000)
	handler := NewHandler(workspace)
	var took []time.Duration
	for range 5 {
		start := time.Now()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/form?path="+relative, nil))
		took = append(took, time.Since(start))
		if response.Code != http.StatusOK {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
	}
	opened, err := workspace.ReadManagedForm(relative)
	if err != nil {
		t.Fatal(err)
	}
	if elements := countElements(opened.Form.Items); elements < 5000 {
		t.Fatalf("the form holds %d elements, not the size of the largest", elements)
	}
	if !hasFormDataPath(opened.DataPaths, "Объект.Оценка") {
		t.Fatalf("the data of the object is not offered: %+v", opened.DataPaths)
	}
	slices.Sort(took)
	if took[2] > budget {
		t.Fatalf("opening the largest form took %v, the budget is %v (all: %v)", took[2], budget, took)
	}
}

// Opening a form reads the description of the object it lies in and nothing
// else of the project: an object beside it that does not read keeps neither
// the form nor the data it offers from opening.
//
// Defect caught: the data a form of a catalog offers the designer found by
// loading the whole project, so that one broken description anywhere leaves
// every form without its data paths - and every opening paying for the
// whole project.
func TestOpeningAFormReadsOnlyTheObjectItLiesIn(t *testing.T) {
	t.Parallel()
	workspace, relative, _ := createManagedFormSource(t)
	broken, err := project.ObjectMetadataPath("catalogs", "Сломанный")
	if err != nil {
		t.Fatal(err)
	}
	writeBigFormFile(t, workspace.root, broken, []byte("format: 1\nid: не идентификатор\n"))
	opened, err := workspace.ReadManagedForm(relative)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Объект.Код", "Объект.ИНН", "Объект.Контакты.Телефон"} {
		if !hasFormDataPath(opened.DataPaths, expected) {
			t.Fatalf("data path %q missing from %+v", expected, opened.DataPaths)
		}
	}
}

// The data a form offers follows the slot of its object that names it: the
// form of the object offers the object with its table parts, a list or a
// choice form the list without them, and a form no slot names nothing.
//
// Defect caught: every form of an object offered the data of the object
// form, its table parts among them, which a list does not hold; a form the
// object does not use offered data it never shows.
func TestTheDataAFormOffersFollowsItsSlot(t *testing.T) {
	t.Parallel()
	workspace, relative, _ := createManagedFormSource(t)
	description, err := project.ObjectMetadataPath("catalogs", "Контрагенты")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(workspace.root, filepath.FromSlash(description)))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := metadata.DecodeCatalog(description, bytes.NewReader(content), mustLayout(t, workspace.root))
	if err != nil {
		t.Fatal(err)
	}
	formFolder := filepath.Base(filepath.Dir(relative))
	for name, test := range map[string]struct {
		slots      metadata.ObjectForms
		want, none []string
	}{
		"объекта": {metadata.ObjectForms{Object: formFolder}, []string{"Объект.Код", "Объект.ИНН", "Объект.Контакты.Телефон"}, nil},
		"списка":  {metadata.ObjectForms{List: formFolder}, []string{"Список.Код", "Список.ИНН"}, []string{"Список.Контакты", "Список.Контакты.Телефон"}},
		"выбора":  {metadata.ObjectForms{Choice: formFolder}, []string{"Список.Наименование"}, []string{"Список.Контакты"}},
		"ничья":   {metadata.ObjectForms{Object: "ДругаяФорма"}, nil, []string{"Объект.Код", "Список.Код"}},
	} {
		catalog.Forms.ObjectForms = test.slots
		var source bytes.Buffer
		if err := metadata.Encode(&source, catalog); err != nil {
			t.Fatal(err)
		}
		paths := workspace.formDataPathsFrom(relative, source.String(), mustLayout(t, workspace.root))
		for _, expected := range test.want {
			if !hasFormDataPath(paths, expected) {
				t.Errorf("%s: %q missing from %+v", name, expected, paths)
			}
		}
		for _, unexpected := range test.none {
			if hasFormDataPath(paths, unexpected) {
				t.Errorf("%s: %q offered: %+v", name, unexpected, paths)
			}
		}
	}
}

func mustLayout(t *testing.T, root string) project.Project {
	t.Helper()
	configuration, err := project.ValidateLayout(root)
	if err != nil {
		t.Fatal(err)
	}
	return configuration
}
