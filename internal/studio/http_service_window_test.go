package studio

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const httpServiceStock = "f3200000-0000-4000-8000-000000000001"

// addHTTPService puts into the project an HTTP service ОбменЗаказами at the
// root orders, of one template and one method, with its module.
func addHTTPService(t *testing.T, root string) {
	t.Helper()
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metadata/http-services/ОбменЗаказами/object.yaml", `format: 1
id: `+httpServiceStock+`
name: ОбменЗаказами
title: {ru: Обмен заказами}
root_url: orders
reuse_sessions: auto
session_max_age: 20
templates:
  - id: f3200000-0000-4000-8000-000000000010
    name: Заказы
    title: {ru: Заказы}
    template: /list
    methods:
      - {id: f3200000-0000-4000-8000-000000000011, name: Получить, title: {ru: Получить}, method: GET, handler: Получить}
`)
	write("metadata/http-services/ОбменЗаказами/МодульСервиса.bsl", "Функция Получить(Запрос)\n\tВозврат Неопределено;\nКонецФункции\n")
}

// TestTheRootOfAnHTTPServiceIsCheckedAsThePrototypeChecksIt catches a root
// taken or refused against the prototype's words (http-servis.md,
// «Проверки»): none - «Корневой URL должен быть заполнен»; a slash, a plus,
// an exclamation, a star, a quote, a space, a percent, a colon -
// «Некорректный корневой URL»; letters, digits and - ~ $ ( ) _ . and
// Cyrillic taken. «.» and «..» the prototype takes and ML refuses (an error
// of the platform not copied, METADATA-OBJECTS.md).
func TestTheRootOfAnHTTPServiceIsCheckedAsThePrototypeChecksIt(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]string{
		"":                "Корневой URL должен быть заполнен",
		"a/b":             "Некорректный корневой URL",
		"a+b":             "Некорректный корневой URL",
		"a!b":             "Некорректный корневой URL",
		"a*b":             "Некорректный корневой URL",
		"a'b":             "Некорректный корневой URL",
		"a b":             "Некорректный корневой URL",
		"a%20b":           "Некорректный корневой URL",
		"a:b":             "Некорректный корневой URL",
		"..":              "Некорректный корневой URL",
		"a$b~c(d)e_f.g-h": "",
		"обмен":           "",
	} {
		if got := rootURLRefusals[metadata.RootURLFaultOf(value)]; got != want {
			t.Errorf("%q answered %q, want %q", value, got, want)
		}
	}
}

// TestTheWindowOfAnHTTPServiceWritesItsSubsystems catches the window of an
// HTTP service losing what it writes: the pages of the kind; the root, the
// sessions and the subsystem marked written; a root the model refuses not
// written; a revision read before refused.
func TestTheWindowOfAnHTTPServiceWritesItsSubsystems(t *testing.T) {
	t.Parallel()
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	addHTTPService(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	const orders = "metadata/http-services/ОбменЗаказами/object.yaml"
	opened, err := workspace.ReadHTTPServiceEditor(orders)
	if err != nil {
		t.Fatal(err)
	}
	var pages []string
	for _, page := range opened.Pages {
		pages = append(pages, page.Title)
	}
	if !slices.Equal(pages, []string{"Основные", "Подсистемы", "Шаблоны URL", "Управление сеансами", "Прочее"}) || opened.KindTitle != "HTTP-сервис" {
		t.Fatalf("the window of an HTTP service has %v (%s)", pages, opened.KindTitle)
	}
	refused := opened.Service
	refused.RootURL = "a/b"
	if _, err := workspace.SaveHTTPServiceWindow(orders, HTTPServiceWindowValue{HTTPServiceDefinition: refused}, opened.Revision); err == nil {
		t.Fatal("a root of a slash was saved")
	}
	edited := opened.Service
	edited.RootURL, edited.SessionMaxAge = "заказы", 90
	marks := []uuid.UUID{mustUUID(t, treeSalesSubsystem)}
	saved, err := workspace.SaveHTTPServiceWindow(orders, HTTPServiceWindowValue{HTTPServiceDefinition: edited, Subsystems: &marks}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Service.RootURL != "заказы" || saved.Service.SessionMaxAge != 90 {
		t.Errorf("saved %+v", saved.Service)
	}
	subsystem, _ := os.ReadFile(filepath.Join(root, "metadata", "subsystems", treeSalesSubsystem+".yaml"))
	if !strings.Contains(string(subsystem), httpServiceStock) {
		t.Errorf("the subsystem does not hold the service: %s", subsystem)
	}
	if _, err := workspace.SaveHTTPServiceWindow(orders, HTTPServiceWindowValue{HTTPServiceDefinition: edited}, opened.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Errorf("a save at the revision read before answered %v", err)
	}
	if _, err := metadata.Load(root); err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
}
