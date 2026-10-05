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
packages: [{package: `+firstXDTOPackage+`}]
descriptor_file: exchange.1cws
reuse_sessions: do-not-use
session_max_age: 20
operations:
  - id: f3000000-0000-4000-8000-000000000010
    name: Проверка
    title: {ru: Проверка соединения}
    procedure: Проверка
    return_type: {namespace: "http://www.w3.org/2001/XMLSchema", name: string}
    nillable: true
    transactioned: false
    data_lock_control: managed
    parameters:
      - id: f3000000-0000-4000-8000-000000000011
        name: ИмяПланаОбмена
        title: {ru: Имя плана обмена}
        type: {namespace: "http://www.w3.org/2001/XMLSchema", name: string}
        direction: in
      - id: f3000000-0000-4000-8000-000000000012
        name: Результат
        title: {ru: Результат}
        type: {namespace: "http://www.w3.org/2001/XMLSchema", name: boolean}
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

// A service describing itself by a package that is gone is an error of the
// project. The strict load refuses the project naming the place; the reading for
// editing opens it and lists the reference to be fixed (owner, 05.10.2026).
func TestWebServicePackageThatIsGoneIsNamed(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeWebService(t, root, `namespace: http://example.org/exchange/1.0
packages: [{package: `+firstXDTOPackage+`}]
`)
	// A package that is gone is an error of the project naming the place
	// (owner, 05.10.2026). Defect caught: the reference accepted silently, or
	// refused without saying where.
	if !hasUnresolved(t, root, "web service ОбменДанными package", firstXDTOPackage) {
		t.Fatal("the package that is gone is not named")
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
      - {id: f3000000-0000-4000-8000-000000000011, name: Имя, title: {ru: Имя}, type: {name: ""}}
`,
		"направление не из набора": `namespace: http://example.org/exchange/1.0
operations:
  - id: f3000000-0000-4000-8000-000000000010
    name: Проверка
    title: {ru: Проверка}
    procedure: Проверка
    parameters:
      - {id: f3000000-0000-4000-8000-000000000011, name: Имя, title: {ru: Имя}, type: {namespace: "http://www.w3.org/2001/XMLSchema", name: string}, direction: назад}
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

// A service is described by packages of the configuration and by namespaces
// the platform provides types in, mixed in one list; and the types of its
// operations are named by namespace and local name, which must both come back.
// The defects caught: a namespace of the platform refused as an unknown
// package - 18 references in the configurations being moved - and a type kept
// as a prefixed string, which loses its namespace for 77 types of 1346 whose
// prefix the prototype declares on the element itself.
func TestWebServiceKeepsPlatformNamespacesAndExpandedTypeNames(t *testing.T) {
	t.Parallel()
	const (
		core     = "http://v8.1c.ru/8.1/data/core"
		exchange = "http://www.1c.ru/CustomerOrders/Exchange"
	)
	root := metadataProject(t)
	writeXDTOPackage(t, root, firstXDTOPackage, "ОбменТоварами", exchange, "")
	writeWebService(t, root, `namespace: http://example.org/exchange/1.0
packages:
  - {package: `+firstXDTOPackage+`}
  - {namespace: "`+core+`"}
operations:
  - id: f3000000-0000-4000-8000-000000000010
    name: Обмен
    title: {ru: Обмен}
    procedure: Обмен
    return_type: {namespace: "`+exchange+`", name: MessageExchange}
    parameters:
      - {id: f3000000-0000-4000-8000-000000000011, name: Сообщение, title: {ru: Сообщение}, type: {namespace: "`+exchange+`", name: MessageExchange}}
      - {id: f3000000-0000-4000-8000-000000000012, name: Безымянный, title: {ru: Безымянный}, type: {name: Local}}
`)
	catalog, err := Load(root)
	if err != nil {
		t.Fatalf("a service described by a namespace of the platform was refused: %v", err)
	}
	service, _ := catalog.WebService("ОбменДанными")
	switch {
	case len(service.Packages) != 2 || service.Packages[0].Package == nil || service.Packages[1].Namespace != core:
		t.Fatalf("the packages did not come back as written: %+v", service.Packages)
	case len(service.Operations) != 1 || service.Operations[0].ReturnType == nil ||
		*service.Operations[0].ReturnType != (XMLTypeName{Namespace: exchange, Name: "MessageExchange"}):
		t.Fatalf("the type of the answer lost its namespace: %+v", service.Operations)
	case service.Operations[0].Parameters[0].Type != (XMLTypeName{Namespace: exchange, Name: "MessageExchange"}):
		t.Fatalf("the type of a parameter lost its namespace: %+v", service.Operations[0].Parameters[0])
	case service.Operations[0].Parameters[1].Type != (XMLTypeName{Name: "Local"}):
		t.Fatalf("a type in no namespace did not come back as one: %+v", service.Operations[0].Parameters[1])
	}
}

// What is refused in the new shape: a local name that still carries a prefix -
// the namespace has its field, and a prefix means nothing outside the file
// that declared it - and a package entry that names both kinds of package, or
// neither, or one namespace twice.
func TestWebServiceTypeAndPackageShapeIsChecked(t *testing.T) {
	t.Parallel()
	operation := func(parameterType string) string {
		return `operations:
  - id: f3000000-0000-4000-8000-000000000010
    name: Проверка
    title: {ru: Проверка}
    procedure: Проверка
    parameters:
      - {id: f3000000-0000-4000-8000-000000000011, name: Имя, title: {ru: Имя}, type: ` + parameterType + `}
`
	}
	for name, test := range map[string]struct{ body, want string }{
		"префикс в локальном имени параметра": {operation(`{name: "xs:string"}`), "is a local name"},
		"префикс в локальном имени ответа": {`operations:
  - {id: f3000000-0000-4000-8000-000000000010, name: Проверка, title: {ru: Проверка}, procedure: Проверка, return_type: {name: "d6p1:Message"}}
`, "is a local name"},
		"пакет назван обоими способами": {`packages:
  - {package: ` + firstXDTOPackage + `, namespace: "http://v8.1c.ru/8.1/data/core"}
`, "names both"},
		"пакет не назван никак": {`packages:
  - {}
`, "must name a package of the configuration or a namespace of the platform"},
		"пространство платформы дважды": {`packages:
  - {namespace: "http://v8.1c.ru/8.1/data/core"}
  - {namespace: "http://v8.1c.ru/8.1/data/core"}
`, "is named twice"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeXDTOPackage(t, root, firstXDTOPackage, "ОбменТоварами", "http://example.org/goods/1.0", "")
			writeWebService(t, root, "namespace: http://example.org/exchange/1.0\n"+test.body)
			_, err := Load(root)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("refused for another reason: %v", err)
			}
		})
	}
}
