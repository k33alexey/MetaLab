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

const (
	webServiceGoods = "f3100000-0000-4000-8000-000000000001"
	xdtoGoods       = "f3100000-0000-4000-8000-000000000002"
)

// addWebService puts into the project an XDTO package and a web service
// Склад using it, of one operation, with its module.
func addWebService(t *testing.T, root string) {
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
	write("metadata/xdto-packages/Товары/object.yaml", "format: 1\nid: "+xdtoGoods+"\nname: Товары\ntitle: {ru: Товары}\nnamespace: http://example.org/goods\n")
	write("metadata/web-services/СкладСервис/object.yaml", `format: 1
id: `+webServiceGoods+`
name: СкладСервис
title: {ru: Склад}
namespace: http://example.org/stock
packages: [{package: `+xdtoGoods+`}, {namespace: "http://v8.1c.ru/8.1/data/core"}]
descriptor_file: stock.1cws
reuse_sessions: auto
session_max_age: 20
operations:
  - id: f3100000-0000-4000-8000-000000000010
    name: Остаток
    title: {ru: Остаток}
    procedure: Остаток
    return_type: {namespace: "http://www.w3.org/2001/XMLSchema", name: string}
`)
	write("metadata/web-services/СкладСервис/МодульСервиса.bsl", "Функция Остаток()\n\tВозврат \"\";\nКонецФункции\n")
}

// TestTheWindowOfAWebServiceWritesItsSubsystems catches the window of a web
// service losing what it writes: the pages and the rows of its window, the
// kind's; the namespaces of its packages; a save of the namespace, the
// sessions and the subsystems marked - written into the subsystem - and a new
// name moving the folder with the module; a revision read before refused.
func TestTheWindowOfAWebServiceWritesItsSubsystems(t *testing.T) {
	t.Parallel()
	root := copyDemoProjectForStudioTest(t)
	addTreeSubsystems(t, root)
	addWebService(t, root)
	workspace, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	const stock = "metadata/web-services/СкладСервис/object.yaml"
	opened, err := workspace.ReadWebServiceEditor(stock)
	if err != nil {
		t.Fatal(err)
	}
	var pages []string
	for _, page := range opened.Pages {
		pages = append(pages, page.Title)
	}
	if !slices.Equal(pages, []string{"Основные", "Подсистемы", "Операции", "Управление сеансами", "Прочее"}) || opened.KindTitle != "Web-сервис" {
		t.Fatalf("the window of a web service has %v (%s)", pages, opened.KindTitle)
	}
	if opened.Packages[mustUUID(t, xdtoGoods)] != "http://example.org/goods" {
		t.Errorf("the packages are %v", opened.Packages)
	}
	warehouse := mustUUID(t, treeWarehouseSubsystem)
	edited := opened.Service
	edited.Namespace, edited.ReuseSessions, edited.SessionMaxAge = "http://example.org/stock/2", metadata.DoNotReuseSession, 60
	marks := []uuid.UUID{warehouse}
	saved, err := workspace.SaveWebServiceWindow(stock, WebServiceWindowValue{WebServiceDefinition: edited, Subsystems: &marks}, opened.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Service.Namespace != "http://example.org/stock/2" || saved.Service.SessionMaxAge != 60 {
		t.Errorf("saved %+v", saved.Service)
	}
	subsystem, err := os.ReadFile(filepath.Join(root, "metadata", "subsystems", treeWarehouseSubsystem+".yaml"))
	if err != nil || !strings.Contains(string(subsystem), webServiceGoods) {
		t.Errorf("the subsystem Склад does not hold the service: %s (%v)", subsystem, err)
	}
	if _, err := workspace.SaveWebServiceWindow(stock, WebServiceWindowValue{WebServiceDefinition: edited}, opened.Revision); !errors.Is(err, ErrSourceChanged) {
		t.Errorf("a save at the revision read before answered %v", err)
	}
	renamed := saved.Service
	renamed.Name = "Остатки"
	moved, err := workspace.SaveWebServiceWindow(saved.Path, WebServiceWindowValue{WebServiceDefinition: renamed}, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Path != "metadata/web-services/Остатки/object.yaml" {
		t.Errorf("the renamed service lies at %s", moved.Path)
	}
	if _, err := os.Stat(filepath.Join(root, "metadata", "web-services", "Остатки", "МодульСервиса.bsl")); err != nil {
		t.Errorf("the module did not follow the name: %v", err)
	}
	if _, err := metadata.Load(root); err != nil {
		t.Fatalf("the project does not load: %v", err)
	}
}

// mustUUID is an identifier written in a test.
func mustUUID(t *testing.T, text string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
