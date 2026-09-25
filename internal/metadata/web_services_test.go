package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const webServiceID = "f3000000-0000-4000-8000-000000000001"

// writeWebService writes one service with its module beside the description.
func writeWebService(t *testing.T, root, body string) {
	t.Helper()
	writeMetadata(t, root, WebServiceKind, webServiceID, `format: 1
id: `+webServiceID+`
name: ОбменДанными
title: {ru: Обмен данными}
`+body)
}

// A service is what it offers: a namespace the outside names its types by, the
// packages describing them, and operations with their parameters. All of it
// survives being written and read back.
func TestWebServiceCarriesItsOperationsAndParameters(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeXDTOPackage(t, root, firstXDTOPackage, "ОбменТоварами", "http://example.org/goods/1.0", "")
	writeWebService(t, root, `namespace: http://example.org/exchange/1.0
packages: [`+firstXDTOPackage+`]
descriptor_file: exchange.1cws
reuse_sessions: do-not-use
session_max_age: 20
operations:
  - id: f3000000-0000-4000-8000-000000000010
    name: Проверка
    title: {ru: Проверка соединения}
    procedure: Проверка
    return_type: xs:string
    nillable: true
    transactioned: false
    data_lock_control: managed
    parameters:
      - id: f3000000-0000-4000-8000-000000000011
        name: ИмяПланаОбмена
        title: {ru: Имя плана обмена}
        type: xs:string
        direction: in
      - id: f3000000-0000-4000-8000-000000000012
        name: Результат
        title: {ru: Результат}
        type: xs:boolean
        direction: out
`)
	modulePath := filepath.Join(root, "metadata", string(WebServiceKind), "ОбменДанными", "МодульСервиса.bsl")
	if err := os.WriteFile(modulePath, []byte("Функция Проверка()\n\tВозврат \"\";\nКонецФункции\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	service, ok := catalog.WebService("обменданными")
	if !ok {
		t.Fatalf("the service is not found by name: %+v", catalog.WebServices)
	}
	switch {
	case service.Namespace != "http://example.org/exchange/1.0" || service.DescriptorFile != "exchange.1cws":
		t.Fatalf("the service lost what it is published as: %+v", service)
	case service.ReuseSessions != DoNotReuseSession || service.SessionMaxAge != 20:
		t.Fatalf("the service lost its session settings: %+v", service)
	case len(service.Packages) != 1:
		t.Fatalf("the service lost the package describing it: %+v", service.Packages)
	case len(service.Operations) != 1 || service.Operations[0].Procedure != "Проверка":
		t.Fatalf("the service lost an operation: %+v", service.Operations)
	case len(service.Operations[0].Parameters) != 2 ||
		service.Operations[0].Parameters[1].Direction != TransferOut:
		t.Fatalf("an operation lost a parameter: %+v", service.Operations[0].Parameters)
	}
}

// A service describing itself by a package that is not there describes itself
// by nothing, and the caller finds out while the other side is already waiting.
func TestWebServicePackageMustBeInTheConfiguration(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWebService(t, root, `namespace: http://example.org/exchange/1.0
packages: [`+firstXDTOPackage+`]
`)
	_, err := Load(root)
	if err == nil {
		t.Fatal("a service described by a package nobody wrote was accepted")
	}
	if !strings.Contains(err.Error(), "which is not in the configuration") {
		t.Fatalf("the error does not say what is wrong: %v", err)
	}
}

// What is checked about an operation is what can be checked here: that it names
// a routine, that its types are named at all, and that two of them do not share
// a name. What a type means is the schema's business.
func TestWebServiceOperationsAreChecked(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"операция без процедуры": `namespace: http://example.org/exchange/1.0
operations:
  - {id: f3000000-0000-4000-8000-000000000010, name: Проверка, title: {ru: Проверка}, procedure: ""}
`,
		"две операции одного имени": `namespace: http://example.org/exchange/1.0
operations:
  - {id: f3000000-0000-4000-8000-000000000010, name: Проверка, title: {ru: Проверка}, procedure: Проверка}
  - {id: f3000000-0000-4000-8000-000000000011, name: проверка, title: {ru: Проверка}, procedure: Проверка2}
`,
		"параметр без типа": `namespace: http://example.org/exchange/1.0
operations:
  - id: f3000000-0000-4000-8000-000000000010
    name: Проверка
    title: {ru: Проверка}
    procedure: Проверка
    parameters:
      - {id: f3000000-0000-4000-8000-000000000011, name: Имя, title: {ru: Имя}, type: ""}
`,
		"направление не из набора": `namespace: http://example.org/exchange/1.0
operations:
  - id: f3000000-0000-4000-8000-000000000010
    name: Проверка
    title: {ru: Проверка}
    procedure: Проверка
    parameters:
      - {id: f3000000-0000-4000-8000-000000000011, name: Имя, title: {ru: Имя}, type: xs:string, direction: назад}
`,
		"пустой ответ, который может быть пустым": `namespace: http://example.org/exchange/1.0
operations:
  - {id: f3000000-0000-4000-8000-000000000010, name: Проверка, title: {ru: Проверка}, procedure: Проверка, nillable: true}
`,
		"описание путём, а не именем файла": `namespace: http://example.org/exchange/1.0
descriptor_file: /srv/services/exchange.1cws
`,
		"повторное использование не из набора": `namespace: http://example.org/exchange/1.0
reuse_sessions: иногда
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeWebService(t, root, body)
			if _, err := Load(root); err == nil {
				t.Fatal("a broken service was accepted")
			}
		})
	}
}
