package metadata

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

func xsName(name string) *XDTOName { return &XDTOName{Namespace: xmlSchemaNamespace, Name: name} }

// schemaLines is a schema as the prototype writes it: the mark of byte
// order, lines ending CR LF, the last one with no line break.
func schemaLines(lines ...string) string { return "\uFEFF" + strings.Join(lines, "\r\n") }

// The two packages made in the configurator for the inventory are written
// as it wrote them (docs/materials/configurator-2026-10-27/series-2.603/
// mlxdto1.xsd, configurator-2026-10-28/v5-xsd/tmp-paket.xsd): a new package
// with an import, a global property of no type, one of an imported type and
// an empty value type; a global attribute named by its local name and value
// types with facets. The global attribute is written without the
// prototype's form="qualified", which XML Schema does not allow on a global
// declaration (paket-xdto.md, «Расхождения с ML»). Defects caught: the
// declarations, the prefixes, the forms of the package, the order of the
// facets, the line breaks or the mark of byte order not the prototype's.
func TestTheSchemaOfAPackageIsThePrototypes(t *testing.T) {
	t.Parallel()
	const soap, sample = "http://schemas.xmlsoap.org/soap/envelope/", "http://www.sample-package.org"
	newPackage := XDTOContent{Namespace: sample, ElementFormQualified: boolPointer(true), AttributeFormQualified: boolPointer(false), Imports: []string{soap},
		Properties: []XDTOProperty{{Name: "ТипЗначения1"}, {Name: "Свойство1", Type: &XDTOName{soap, "Body"}}},
		ValueTypes: []XDTOValueType{{Name: "ТипЗначения1"}}}
	soapPackage := XDTOContent{Namespace: soap, ObjectTypes: []XDTOObjectType{{Name: "Body", Open: boolPointer(true), Sequenced: boolPointer(true)}}}
	catalog := xdtoCatalog(newPackage, soapPackage)
	schema, issues := ExportXDTOSchema(newPackage, catalog)
	if want := schemaLines(
		`<xs:schema xmlns:ns1="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tns="http://www.sample-package.org" xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.sample-package.org" attributeFormDefault="unqualified" elementFormDefault="qualified">`,
		"\t"+`<xs:import namespace="http://schemas.xmlsoap.org/soap/envelope/"/>`,
		"\t"+`<xs:element name="ТипЗначения1"/>`,
		"\t"+`<xs:element name="Свойство1" type="ns1:Body"/>`,
		"\t"+`<xs:simpleType name="ТипЗначения1">`,
		"\t\t"+`<xs:restriction/>`,
		"\t"+`</xs:simpleType>`,
		`</xs:schema>`); string(schema) != want || issues != nil {
		t.Errorf("mlxdto1:\n%s\nissues %v", schema, issues)
	}

	facets := XDTOContent{Namespace: sample,
		Properties: []XDTOProperty{{Name: "Свойство1", LocalName: "locName", Form: "Attribute", Type: &XDTOName{sample, "ТипЗначения1"}}},
		ValueTypes: []XDTOValueType{{Name: "ТипЗначения1", Base: xsName("decimal"), WhiteSpace: "collapse", MinInclusive: ptr("1"), MaxInclusive: ptr("100")},
			{Name: "ТипЗначения2", Base: xsName("decimal"), MinExclusive: ptr("0"), MaxExclusive: ptr("10")}}}
	schema, issues = ExportXDTOSchema(facets, xdtoCatalog(facets))
	if want := schemaLines(
		`<xs:schema xmlns:tns="http://www.sample-package.org" xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.sample-package.org" attributeFormDefault="unqualified" elementFormDefault="qualified">`,
		"\t"+`<xs:attribute name="locName" type="tns:ТипЗначения1"/>`,
		"\t"+`<xs:simpleType name="ТипЗначения1">`,
		"\t\t"+`<xs:restriction base="xs:decimal">`,
		"\t\t\t"+`<xs:whiteSpace value="collapse"/>`,
		"\t\t\t"+`<xs:minInclusive value="1"/>`,
		"\t\t\t"+`<xs:maxInclusive value="100"/>`,
		"\t\t"+`</xs:restriction>`,
		"\t"+`</xs:simpleType>`,
		"\t"+`<xs:simpleType name="ТипЗначения2">`,
		"\t\t"+`<xs:restriction base="xs:decimal">`,
		"\t\t\t"+`<xs:minExclusive value="0"/>`,
		"\t\t\t"+`<xs:maxExclusive value="10"/>`,
		"\t\t"+`</xs:restriction>`,
		"\t"+`</xs:simpleType>`,
		`</xs:schema>`); string(schema) != want || issues != nil {
		t.Errorf("tmp-paket:\n%s\nissues %v", schema, issues)
	}
}

// xdtoCatalog is a project of the packages given.
func xdtoCatalog(packages ...XDTOContent) *Catalog {
	catalog := &Catalog{xdtoContents: map[uuid.UUID]XDTOContent{}}
	for index, content := range packages {
		id, _ := uuid.Parse(fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1))
		catalog.XDTOPackages = append(catalog.XDTOPackages, XDTOPackageDefinition{ID: id, Namespace: content.Namespace})
		catalog.xdtoContents[id] = content
	}
	return catalog
}

// Every construction the schemas of the configurator show (paket-xdto.md,
// «Досъёмка 28.10.2026»; the schemas of five packages of ERP were matched
// byte by byte when this was written), each written out here by its rule,
// not by the code: elements nillable, then minOccurs, then maxOccurs, a
// bound of -1 unbounded; a type of no type written as none; an attribute of
// no lower bound required, of 0 not; an object type sequenced and not
// ordered a choice, and a type extending it too; an open one any element and
// any attribute after its own; an abstract one marked; a mixed anonymous
// one; text and attributes alone a simple content; an extension of nothing
// more empty; a reference and a local name; a list, a union, length facets,
// a pattern, digits and enumerations in the prototype's order. Defects
// caught: any of these written another way.
func TestTheSchemaOfAPackageWritesEveryConstruction(t *testing.T) {
	t.Parallel()
	const own, other = "urn:ml:own", "urn:ml:other"
	ownName := func(name string) *XDTOName { return &XDTOName{own, name} }
	content := XDTOContent{Namespace: own, ElementFormQualified: boolPointer(false), AttributeFormQualified: boolPointer(true), Imports: []string{other},
		Properties: []XDTOProperty{{Name: "Документ", Type: ownName("Документ")}},
		ValueTypes: []XDTOValueType{
			{Name: "Код", Base: xsName("string"), MinLength: ptr(1), MaxLength: ptr(10), Patterns: []string{"[0-9]+"},
				Enumerations: []XDTOEnumeration{{Value: "001"}, {Value: "002"}}},
			{Name: "Сумма", Base: xsName("decimal"), MinInclusive: ptr("0"), TotalDigits: ptr(15), FractionDigits: ptr(2)},
			{Name: "Коды", Variety: "List", ItemType: ownName("Код")},
			{Name: "Значение", Variety: "Union", MemberTypes: []XDTOName{{own, "Код"}, {xmlSchemaNamespace, "int"}}},
		},
		ObjectTypes: []XDTOObjectType{
			{Name: "База", Abstract: boolPointer(true), Open: boolPointer(true), Sequenced: boolPointer(true), Ordered: boolPointer(false), Properties: []XDTOProperty{
				{Name: "Имя", Type: xsName("string"), Nillable: boolPointer(true), LowerBound: ptr(0), UpperBound: ptr(-1)},
				{Ref: &XDTOName{other, "Внешний"}, LowerBound: ptr(0)},
			}},
			{Name: "Документ", Base: ownName("База"), Properties: []XDTOProperty{
				{Name: "Номер", LocalName: "number", Type: xsName("int"), UpperBound: ptr(5)},
				{Name: "Признак", Form: "Attribute", Type: xsName("boolean")},
				{Name: "Пометка", Form: "Attribute", Type: xsName("boolean"), LowerBound: ptr(0)},
				{Name: "Сведения", TypeDef: &XDTOTypeDef{Object: &XDTOObjectType{Mixed: boolPointer(true), Properties: []XDTOProperty{
					{Name: "Любое", Type: xsName("anyType")}}}}},
			}},
			{Name: "Строка", Properties: []XDTOProperty{
				{Name: "Вид", Form: "Attribute", LowerBound: ptr(0), TypeDef: &XDTOTypeDef{Value: &XDTOValueType{Base: xsName("string"), MaxLength: ptr(2)}}},
				{Name: "__content", Form: "Text", Type: xsName("string")},
			}},
			{Name: "Пустой", Base: &XDTOName{other, "Внешний"}},
		}}
	otherPackage := XDTOContent{Namespace: other, ObjectTypes: []XDTOObjectType{{Name: "Внешний"}}}
	schema, issues := ExportXDTOSchema(content, xdtoCatalog(content, otherPackage))
	want := schemaLines(
		`<xs:schema xmlns:ns1="urn:ml:other" xmlns:tns="urn:ml:own" xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:ml:own" attributeFormDefault="qualified" elementFormDefault="unqualified">`,
		"\t"+`<xs:import namespace="urn:ml:other"/>`,
		"\t"+`<xs:element name="Документ" type="tns:Документ"/>`,
		"\t"+`<xs:simpleType name="Код">`,
		"\t\t"+`<xs:restriction base="xs:string">`,
		"\t\t\t"+`<xs:minLength value="1"/>`,
		"\t\t\t"+`<xs:maxLength value="10"/>`,
		"\t\t\t"+`<xs:pattern value="[0-9]+"/>`,
		"\t\t\t"+`<xs:enumeration value="001"/>`,
		"\t\t\t"+`<xs:enumeration value="002"/>`,
		"\t\t"+`</xs:restriction>`,
		"\t"+`</xs:simpleType>`,
		"\t"+`<xs:simpleType name="Сумма">`,
		"\t\t"+`<xs:restriction base="xs:decimal">`,
		"\t\t\t"+`<xs:minInclusive value="0"/>`,
		"\t\t\t"+`<xs:totalDigits value="15"/>`,
		"\t\t\t"+`<xs:fractionDigits value="2"/>`,
		"\t\t"+`</xs:restriction>`,
		"\t"+`</xs:simpleType>`,
		"\t"+`<xs:simpleType name="Коды">`,
		"\t\t"+`<xs:list itemType="tns:Код"/>`,
		"\t"+`</xs:simpleType>`,
		"\t"+`<xs:simpleType name="Значение">`,
		"\t\t"+`<xs:union memberTypes="tns:Код xs:int"/>`,
		"\t"+`</xs:simpleType>`,
		"\t"+`<xs:complexType name="База" abstract="true">`,
		"\t\t"+`<xs:choice>`,
		"\t\t\t"+`<xs:element name="Имя" type="xs:string" nillable="true" minOccurs="0" maxOccurs="unbounded"/>`,
		"\t\t\t"+`<xs:element ref="ns1:Внешний" minOccurs="0"/>`,
		"\t\t\t"+`<xs:any namespace="##any" processContents="lax" minOccurs="0" maxOccurs="unbounded"/>`,
		"\t\t"+`</xs:choice>`,
		"\t\t"+`<xs:anyAttribute namespace="##any" processContents="lax"/>`,
		"\t"+`</xs:complexType>`,
		"\t"+`<xs:complexType name="Документ">`,
		"\t\t"+`<xs:complexContent>`,
		"\t\t\t"+`<xs:extension base="tns:База">`,
		"\t\t\t\t"+`<xs:choice>`,
		"\t\t\t\t\t"+`<xs:element name="number" type="xs:int" maxOccurs="5"/>`,
		"\t\t\t\t\t"+`<xs:element name="Сведения">`,
		"\t\t\t\t\t\t"+`<xs:complexType mixed="true">`,
		"\t\t\t\t\t\t\t"+`<xs:sequence>`,
		"\t\t\t\t\t\t\t\t"+`<xs:element name="Любое"/>`,
		"\t\t\t\t\t\t\t"+`</xs:sequence>`,
		"\t\t\t\t\t\t"+`</xs:complexType>`,
		"\t\t\t\t\t"+`</xs:element>`,
		"\t\t\t\t"+`</xs:choice>`,
		"\t\t\t\t"+`<xs:attribute name="Признак" type="xs:boolean" use="required"/>`,
		"\t\t\t\t"+`<xs:attribute name="Пометка" type="xs:boolean"/>`,
		"\t\t\t"+`</xs:extension>`,
		"\t\t"+`</xs:complexContent>`,
		"\t"+`</xs:complexType>`,
		"\t"+`<xs:complexType name="Строка">`,
		"\t\t"+`<xs:simpleContent>`,
		"\t\t\t"+`<xs:extension base="xs:string">`,
		"\t\t\t\t"+`<xs:attribute name="Вид">`,
		"\t\t\t\t\t"+`<xs:simpleType>`,
		"\t\t\t\t\t\t"+`<xs:restriction base="xs:string">`,
		"\t\t\t\t\t\t\t"+`<xs:maxLength value="2"/>`,
		"\t\t\t\t\t\t"+`</xs:restriction>`,
		"\t\t\t\t\t"+`</xs:simpleType>`,
		"\t\t\t\t"+`</xs:attribute>`,
		"\t\t\t"+`</xs:extension>`,
		"\t\t"+`</xs:simpleContent>`,
		"\t"+`</xs:complexType>`,
		"\t"+`<xs:complexType name="Пустой">`,
		"\t\t"+`<xs:complexContent>`,
		"\t\t\t"+`<xs:extension base="ns1:Внешний"/>`,
		"\t\t"+`</xs:complexContent>`,
		"\t"+`</xs:complexType>`,
		`</xs:schema>`)
	if string(schema) != want || issues != nil {
		got, expected := strings.Split(string(schema), "\r\n"), strings.Split(want, "\r\n")
		for index := range expected {
			if index >= len(got) || got[index] != expected[index] {
				line := "<end>"
				if index < len(got) {
					line = got[index]
				}
				t.Fatalf("line %d:\n got %s\nwant %s\nissues %v", index+1, line, expected[index], issues)
			}
		}
		t.Fatalf("longer than wanted: %d lines, issues %v", len(got), issues)
	}
}

// A package the prototype's check finds wrong is not exported (paket-xdto.md,
// «Проверить пакет»; snapshots series-2.603/22, 23, 26): the type of a global
// property from a namespace not imported, two members of a union from it -
// one issue each - and, with the namespace imported, two members that are
// object types. The codes and the words are the prototype's, copied here. A
// type of a property of a type from a namespace not imported cannot be
// written either. Defects caught: a wrong package exported, an issue not
// found, the prototype's code or words changed.
func TestAPackageTheCheckRefusesIsNotExported(t *testing.T) {
	t.Parallel()
	const soap, sample = "http://schemas.xmlsoap.org/soap/envelope/", "http://www.sample-package.org"
	soapPackage := XDTOContent{Namespace: soap, ObjectTypes: []XDTOObjectType{{Name: "Body"}, {Name: "Envelope"}}}
	union := XDTOValueType{Name: "ТипЗначения1", Variety: "Union", MemberTypes: []XDTOName{{soap, "Body"}, {soap, "Envelope"}}}
	wrong := XDTOContent{Namespace: sample, Properties: []XDTOProperty{{Name: "Свойство1", Type: &XDTOName{soap, "Body"}}}, ValueTypes: []XDTOValueType{union}}
	schema, issues := ExportXDTOSchema(wrong, xdtoCatalog(wrong, soapPackage))
	want := []XDTOSchemaIssue{
		{Code: "xdto-package-4.4", Text: "Тип глобального свойства 'Свойство1' не найден в пакете типов 'http://www.sample-package.org' и его зависимостях"},
		{Code: "xdto-valueType-4.1", Type: "ТипЗначения1", Text: "Тип объединения '{http://schemas.xmlsoap.org/soap/envelope/}Body' отсутствует или не определен"},
		{Code: "xdto-valueType-4.1", Type: "ТипЗначения1", Text: "Тип объединения '{http://schemas.xmlsoap.org/soap/envelope/}Envelope' отсутствует или не определен"},
	}
	if schema != nil || !equalIssues(issues, want) {
		t.Errorf("not imported: %v, %d bytes", issues, len(schema))
	}
	imported := wrong
	imported.Imports, imported.Properties = []string{soap}, nil
	schema, issues = ExportXDTOSchema(imported, xdtoCatalog(imported, soapPackage))
	members := XDTOSchemaIssue{Code: "xdto-valueType-4.1.2", Type: "ТипЗначения1", Text: "Тип объединения должен являться типом значения"}
	if schema != nil || !equalIssues(issues, []XDTOSchemaIssue{members, members}) {
		t.Errorf("object types in a union: %v", issues)
	}
	nested := XDTOContent{Namespace: sample, ObjectTypes: []XDTOObjectType{{Name: "Т", Properties: []XDTOProperty{{Name: "С", Type: &XDTOName{soap, "Body"}}}}}}
	if schema, issues = ExportXDTOSchema(nested, xdtoCatalog(nested, soapPackage)); schema != nil || len(issues) != 1 {
		t.Errorf("a type of a property of a type not imported: %v", issues)
	}
	fine := imported
	fine.ValueTypes = []XDTOValueType{{Name: "ТипЗначения1", Variety: "Union", MemberTypes: []XDTOName{{xmlSchemaNamespace, "string"}}}}
	if schema, issues = ExportXDTOSchema(fine, xdtoCatalog(fine, soapPackage)); schema == nil || issues != nil {
		t.Errorf("a right package was refused: %v", issues)
	}
}

func equalIssues(got, want []XDTOSchemaIssue) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
