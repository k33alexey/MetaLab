package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	httpServiceID       = "f4000000-0000-4000-8000-000000000001"
	secondHTTPServiceID = "f4000000-0000-4000-8000-000000000002"
)

func writeHTTPService(t *testing.T, root, id, name, body string) {
	t.Helper()
	writeMetadata(t, root, HTTPServiceKind, id, `format: 1
id: `+id+`
name: `+name+`
title: {ru: `+name+`}
`+body)
}

// A service is a root address, the addresses under it and the verbs each of
// them answers. All three survive being written and read back.
func TestHTTPServiceCarriesItsTemplatesAndMethods(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeHTTPService(t, root, httpServiceID, "Биллинг", `root_url: billing
reuse_sessions: auto
session_max_age: 20
templates:
  - id: f4000000-0000-4000-8000-000000000010
    name: Версия
    title: {ru: Версия}
    template: /version
    methods:
      - {id: f4000000-0000-4000-8000-000000000011, name: Получить, title: {ru: Получить}, method: GET, handler: ВерсияПолучить}
  - id: f4000000-0000-4000-8000-000000000020
    name: СчетНаОплату
    title: {ru: Счёт на оплату}
    template: /bill/{Версия}/*
    methods:
      - {id: f4000000-0000-4000-8000-000000000021, name: Добавить, title: {ru: Добавить}, method: POST, handler: СчетДобавить}
      - {id: f4000000-0000-4000-8000-000000000022, name: Изменить, title: {ru: Изменить}, method: PUT, handler: СчетИзменить}
`)
	modulePath := filepath.Join(root, "metadata", string(HTTPServiceKind), "Биллинг", "МодульСервиса.bsl")
	if err := os.WriteFile(modulePath, []byte("Функция ВерсияПолучить(Запрос)\n\tВозврат Неопределено;\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	service, ok := catalog.HTTPService("биллинг")
	if !ok {
		t.Fatalf("the service is not found by name: %+v", catalog.HTTPServices)
	}
	switch {
	case service.RootURL != "billing" || service.ReuseSessions != ReuseSessionAutomatic || service.SessionMaxAge != 20:
		t.Fatalf("the service lost where it answers: %+v", service)
	case len(service.Templates) != 2:
		t.Fatalf("an address was lost: %+v", service.Templates)
	case service.Templates[1].Template != "/bill/{Версия}/*":
		t.Fatalf("the address came back as %q", service.Templates[1].Template)
	case len(service.Templates[1].Methods) != 2 || service.Templates[1].Methods[1].Handler != "СчетИзменить":
		t.Fatalf("a method was lost: %+v", service.Templates[1].Methods)
	}
}

// Two services answering at one root are two answers to every call that reaches
// it, and the caller gets whichever the server resolved first.
func TestTwoHTTPServicesCannotShareARootAddress(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeHTTPService(t, root, httpServiceID, "Биллинг", "root_url: billing\n")
	writeHTTPService(t, root, secondHTTPServiceID, "БиллингНовый", "root_url: Billing\n")
	_, err := Load(root)
	if err == nil {
		t.Fatal("two services on one root address were accepted")
	}
	if !strings.Contains(err.Error(), "both answer at") {
		t.Fatalf("the error does not say what is wrong: %v", err)
	}
}

// What is checked about an address is what makes it answer a call at all.
func TestHTTPServiceAddressesAreChecked(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"без корневого адреса": "templates: []\n",
		"корень путём":         "root_url: billing/v1\n",
		"адрес без косой черты": `root_url: billing
templates:
  - {id: f4000000-0000-4000-8000-000000000010, name: Версия, title: {ru: Версия}, template: version}
`,
		"незакрытый параметр": `root_url: billing
templates:
  - {id: f4000000-0000-4000-8000-000000000010, name: Версия, title: {ru: Версия}, template: "/bill/{Версия"}
`,
		"параметр в параметре": `root_url: billing
templates:
  - {id: f4000000-0000-4000-8000-000000000010, name: Версия, title: {ru: Версия}, template: "/bill/{{Версия}}"}
`,
		"два шаблона на одном адресе": `root_url: billing
templates:
  - {id: f4000000-0000-4000-8000-000000000010, name: Версия, title: {ru: Версия}, template: /version}
  - {id: f4000000-0000-4000-8000-000000000020, name: ВерсияВтораяа, title: {ru: Версия}, template: /version}
`,
		"глагол не из протокола": `root_url: billing
templates:
  - id: f4000000-0000-4000-8000-000000000010
    name: Версия
    title: {ru: Версия}
    template: /version
    methods:
      - {id: f4000000-0000-4000-8000-000000000011, name: Получить, title: {ru: Получить}, method: GTE, handler: Получить}
`,
		"один глагол дважды на адресе": `root_url: billing
templates:
  - id: f4000000-0000-4000-8000-000000000010
    name: Версия
    title: {ru: Версия}
    template: /version
    methods:
      - {id: f4000000-0000-4000-8000-000000000011, name: Получить, title: {ru: Получить}, method: GET, handler: Получить}
      - {id: f4000000-0000-4000-8000-000000000012, name: ПолучитьЕще, title: {ru: Получить}, method: GET, handler: ПолучитьЕще}
`,
		"метод без обработчика": `root_url: billing
templates:
  - id: f4000000-0000-4000-8000-000000000010
    name: Версия
    title: {ru: Версия}
    template: /version
    methods:
      - {id: f4000000-0000-4000-8000-000000000011, name: Получить, title: {ru: Получить}, method: GET, handler: ""}
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeHTTPService(t, root, httpServiceID, "Биллинг", body)
			if _, err := Load(root); err == nil {
				t.Fatal("a broken service was accepted")
			}
		})
	}
}
