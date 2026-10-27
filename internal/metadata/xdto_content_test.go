package metadata

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// xdtoSample is a body holding a node of every kind and every value the
// editor of a package shows (paket-xdto.md, «Узлы редактора»; help,
// СвойствоXDTO, ТипЗначенияXDTO, ТипОбъектаXDTO, ВидФасетаXDTO), written as
// the prototype writes it: the order of the attributes of the exports, a
// prefix «d<depth>p<n>» declared on the node naming a namespace and reused
// below it, the members of a union in braces, «&», «<», «>» and the quote
// escaped, an empty enumeration closed on itself.
var xdtoSample = xdtoBody("http://example.org/goods", ""+
	"\t<import namespace=\"http://v8.1c.ru/8.1/data/core\"/>\r\n"+
	"\t<property xmlns:d2p1=\"http://v8.1c.ru/8.1/data/core\" name=\"Корень\" type=\"d2p1:UUID\" lowerBound=\"0\" fixed=\"true\" upperBound=\"-1\" default=\"a&amp;b &lt;c&gt; &quot;d&quot;\" nillable=\"false\" form=\"Attribute\" localName=\"root\" qualified=\"true\"/>\r\n"+
	"\t<property name=\"Код\">\r\n"+
	"\t\t<typeDef xsi:type=\"ValueType\" base=\"xs:string\" variety=\"Atomic\" maxLength=\"10\">\r\n"+
	"\t\t\t<pattern>[0-9]+&amp;</pattern>\r\n"+
	"\t\t</typeDef>\r\n"+
	"\t</property>\r\n"+
	"\t<valueType name=\"Цена\" base=\"xs:decimal\" variety=\"Atomic\" length=\"5\" minLength=\"1\" maxLength=\"9\" whiteSpace=\"collapse\" minInclusive=\"0\" minExclusive=\"-1\" maxInclusive=\"100\" maxExclusive=\"101\" totalDigits=\"15\" fractionDigits=\"2\"/>\r\n"+
	"\t<valueType name=\"Вид\" base=\"xs:string\">\r\n"+
	"\t\t<enumeration xsi:type=\"xs:string\">Товар</enumeration>\r\n"+
	"\t\t<enumeration>Услуга</enumeration>\r\n"+
	"\t\t<enumeration/>\r\n"+
	"\t</valueType>\r\n"+
	"\t<valueType name=\"Уровни\" variety=\"List\" itemType=\"xs:int\"/>\r\n"+
	"\t<valueType name=\"Ключ\" variety=\"Union\" memberTypes=\"{http://v8.1c.ru/8.1/data/core}UUID {http://www.w3.org/2001/XMLSchema}string\">\r\n"+
	"\t\t<typeDef xsi:type=\"ValueType\" base=\"xs:int\"/>\r\n"+
	"\t</valueType>\r\n"+
	"\t<objectType xmlns:d2p1=\"http://example.org/base\" name=\"Товар\" base=\"d2p1:Объект\" open=\"true\" abstract=\"false\" mixed=\"true\" ordered=\"false\" sequenced=\"true\">\r\n"+
	"\t\t<property name=\"Владелец\" type=\"d2p1:Владелец\" lowerBound=\"0\" nillable=\"true\"/>\r\n"+
	"\t\t<property xmlns:d3p1=\"http://example.org/goods\" ref=\"d3p1:Корень\"/>\r\n"+
	"\t\t<property name=\"Строки\" upperBound=\"-1\">\r\n"+
	"\t\t\t<typeDef xmlns:d4p1=\"http://example.org/other\" xsi:type=\"ObjectType\" base=\"d4p1:Строка\">\r\n"+
	"\t\t\t\t<property name=\"Номер\" type=\"xs:int\"/>\r\n"+
	"\t\t\t</typeDef>\r\n"+
	"\t\t</property>\r\n"+
	"\t</objectType>\r\n")

// The body of a package is read into the model of XDTO and written back as
// the prototype writes it (paket-xdto.md; the 294 bodies of the exports,
// probed: every one read, every one read the same after writing, 286 written
// byte for byte, the other eight in an earlier form of the members of a
// union). The expectation is written out here, not taken from the code.
// Defects caught: a value of a node lost or read as another; a value not
// said read as said, or the other way round (the editor shows the first
// greyed and does not write it); a name of a type resolved by the wrong
// namespace; the nodes, the attributes or the prefixes written out of the
// prototype's order or form; text not escaped.
func TestTheBodyOfAPackageIsTheModelOfXDTO(t *testing.T) {
	content, err := DecodeXDTOContent(strings.NewReader(xdtoSample))
	if err != nil {
		t.Fatal(err)
	}
	yes, no, zero, minus, one, two, five, nine, ten, fifteen := true, false, 0, -1, 1, 2, 5, 9, 10, 15
	text := func(value string) *string { return &value }
	core := func(name string) *XDTOName { return &XDTOName{Namespace: "http://v8.1c.ru/8.1/data/core", Name: name} }
	xs := func(name string) *XDTOName {
		return &XDTOName{Namespace: "http://www.w3.org/2001/XMLSchema", Name: name}
	}
	want := XDTOContent{
		Namespace: "http://example.org/goods",
		Imports:   []string{"http://v8.1c.ru/8.1/data/core"},
		Properties: []XDTOProperty{
			{Name: "Корень", Type: core("UUID"), LowerBound: &zero, Fixed: &yes, UpperBound: &minus, Default: text(`a&b <c> "d"`),
				Nillable: &no, Form: "Attribute", LocalName: "root", Qualified: &yes},
			{Name: "Код", TypeDef: &XDTOTypeDef{Value: &XDTOValueType{Base: xs("string"), Variety: "Atomic", MaxLength: &ten, Patterns: []string{"[0-9]+&"}}}},
		},
		ValueTypes: []XDTOValueType{
			{Name: "Цена", Base: xs("decimal"), Variety: "Atomic", Length: &five, MinLength: &one, MaxLength: &nine, WhiteSpace: "collapse",
				MinInclusive: text("0"), MinExclusive: text("-1"), MaxInclusive: text("100"), MaxExclusive: text("101"), TotalDigits: &fifteen, FractionDigits: &two},
			{Name: "Вид", Base: xs("string"), Enumerations: []XDTOEnumeration{{Type: xs("string"), Value: "Товар"}, {Value: "Услуга"}, {}}},
			{Name: "Уровни", Variety: "List", ItemType: xs("int")},
			{Name: "Ключ", Variety: "Union", MemberTypes: []XDTOName{*core("UUID"), *xs("string")}, TypeDefs: []XDTOTypeDef{{Value: &XDTOValueType{Base: xs("int")}}}},
		},
		ObjectTypes: []XDTOObjectType{{Name: "Товар", Base: &XDTOName{Namespace: "http://example.org/base", Name: "Объект"},
			Open: &yes, Abstract: &no, Mixed: &yes, Ordered: &no, Sequenced: &yes, Properties: []XDTOProperty{
				{Name: "Владелец", Type: &XDTOName{Namespace: "http://example.org/base", Name: "Владелец"}, LowerBound: &zero, Nillable: &yes},
				{Ref: &XDTOName{Namespace: "http://example.org/goods", Name: "Корень"}},
				{Name: "Строки", UpperBound: &minus, TypeDef: &XDTOTypeDef{Object: &XDTOObjectType{Base: &XDTOName{Namespace: "http://example.org/other", Name: "Строка"},
					Properties: []XDTOProperty{{Name: "Номер", Type: xs("int")}}}}},
			}}},
	}
	got, _ := json.Marshal(content)
	expected, _ := json.Marshal(want)
	if !bytes.Equal(got, expected) {
		t.Errorf("read:\n%s\nwant\n%s", got, expected)
	}
	var written bytes.Buffer
	if err := EncodeXDTOContent(&written, content); err != nil {
		t.Fatal(err)
	}
	if written.String() != xdtoSample {
		t.Errorf("written:\n%s\nwant\n%s", written.String(), xdtoSample)
	}
	var empty bytes.Buffer
	if err := EncodeXDTOContent(&empty, XDTOContent{Namespace: "http://example.org/empty", ElementFormQualified: &yes, AttributeFormQualified: &no}); err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(xdtoBody("http://example.org/empty", ""), "/>", ` elementFormQualified="true" attributeFormQualified="false"/>`, 1); empty.String() != want {
		t.Errorf("an empty package: %s, want %s", empty.String(), want)
	}
}

// A body that is not one is refused with what is wrong: not XML, a root
// other than the model's package, a flag not true or false, a bound not a
// whole number, a name of a type by a prefix nobody declared, a definition
// of a type that is neither, text beside the nodes.
// Defects caught: such a body read as an empty package or with the value
// dropped - a loss nothing reports.
func TestABodyNotOfTheModelOfXDTOIsRefused(t *testing.T) {
	for _, body := range map[string]string{
		"not XML":    "<package",
		"root":       `<schema xmlns="http://www.w3.org/2001/XMLSchema"/>`,
		"flag":       xdtoBody("u", "\t<objectType name=\"A\" open=\"yes\"/>\r\n"),
		"bound":      xdtoBody("u", "\t<property name=\"A\" lowerBound=\"one\"/>\r\n"),
		"prefix":     xdtoBody("u", "\t<property name=\"A\" type=\"d9p1:B\"/>\r\n"),
		"definition": xdtoBody("u", "\t<property name=\"A\">\r\n\t\t<typeDef xsi:type=\"Other\"/>\r\n\t</property>\r\n"),
		"text":       xdtoBody("u", "\tloose\r\n\t<import namespace=\"a\"/>\r\n"),
	} {
		if _, err := DecodeXDTOContent(strings.NewReader(body)); err == nil {
			t.Errorf("accepted: %s", body)
		}
	}
}

// What the model of XDTO does not name - an attribute of a node, an element
// inside one - is carried as written, written back beside its node and noted
// (the iron rule, CONFORMANCE.md).
// Defects caught: such a value dropped on reading or on writing, or kept
// without a note.
func TestWhatTheModelOfXDTODoesNotKnowIsCarriedAndNoted(t *testing.T) {
	body := xdtoBody("u", "\t<objectType name=\"A\" color=\"red\">\r\n\t\t<property name=\"B\" type=\"xs:int\"/>\r\n\t\t<annotation>x</annotation>\r\n\t</objectType>\r\n")
	content, err := DecodeXDTOContent(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	unknown := content.ObjectTypes[0].Unknown
	if unknown.empty() || len(unknown.Attributes) != 1 || unknown.Attributes[0].Value != "red" || len(unknown.Elements) != 1 {
		t.Fatalf("not carried: %+v", unknown)
	}
	var written bytes.Buffer
	if err := EncodeXDTOContent(&written, content); err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(body, "\t\t<annotation>", "\t\t<annotation>", 1); written.String() != want {
		t.Errorf("written:\n%q\nwant\n%q", written.String(), want)
	}
	var notes []string
	content.unknownPlaces(func(where, written string) { notes = append(notes, where+": "+written) })
	if len(notes) != 1 || notes[0] != `, тип объекта A: color="red" <annotation>x</annotation>` {
		t.Errorf("noted: %q", notes)
	}
}

// The body is read with the project and names the namespace of its
// package again: a body of another namespace is refused on load (the 294
// packages of the exports agree).
// Defects caught: two places naming the namespace of a package drifting
// apart unseen; the body not read with the project.
func TestTheBodyOfAPackageIsReadWithTheProject(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeXDTOPackage(t, root, firstXDTOPackage, "Обмен", "http://example.org/goods", xdtoSample)
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := catalog.XDTOPackage("Обмен")
	content, ok := catalog.XDTOPackageContent(item.ID)
	if !ok || len(content.ObjectTypes) != 1 || content.ObjectTypes[0].Name != "Товар" {
		t.Fatalf("the body is not read with the project: %+v", content)
	}
	content.ObjectTypes[0].Name = "Изменён"
	if again, _ := catalog.XDTOPackageContent(item.ID); again.ObjectTypes[0].Name != "Товар" {
		t.Error("the body given out is the catalog's own")
	}
	other := metadataProject(t)
	writeXDTOPackage(t, other, firstXDTOPackage, "Обмен", "http://example.org/other", xdtoSample)
	if _, err := Load(other); err == nil || !strings.Contains(err.Error(), "the body defines namespace") {
		t.Errorf("a body of another namespace: %v", err)
	}
}

// The types a type is chosen from (web-servis.md, «Выбор типа из палитры
// параметра»; configurator-2026-10-27/series-2.595/notes.md, В9): XML
// Schema's 91 from «all» to «wildcard» by the alphabet whatever the case,
// data/core's 46 from «AllowedLength» to «ValueTreeRow», a package's own
// value types and object types by the alphabet whatever the case, a
// package of no body none.
// Defects caught: a type missing or out of the window's order; the types
// of a package not read from its body, or read in the order written.
func TestTheTypesOfTheNamespacesAreTheWindows(t *testing.T) {
	t.Parallel()
	root := metadataProject(t)
	writeXDTOPackage(t, root, firstXDTOPackage, "Обмен", "http://example.org/goods", xdtoBody("http://example.org/goods",
		"\t<valueType name=\"цена\"/>\r\n\t<valueType name=\"Вид\"/>\r\n\t<objectType name=\"Товар\"/>\r\n\t<objectType name=\"Адрес\"/>\r\n"))
	writeXDTOPackage(t, root, secondXDTOPackage, "Пустой", "http://example.org/empty", "")
	catalog, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	types := catalog.XDTOTypes()
	schema := types["http://www.w3.org/2001/XMLSchema"]
	if len(schema) != 91 || schema[0] != "all" || schema[90] != "wildcard" || schema[24] != "ENTITY" || schema[78] != "string" {
		t.Errorf("XML Schema: %d %q", len(schema), schema)
	}
	core := types["http://v8.1c.ru/8.1/data/core"]
	if len(core) != 46 || core[0] != "AllowedLength" || core[45] != "ValueTreeRow" {
		t.Errorf("data/core: %d %q", len(core), core)
	}
	// The other namespaces of the platform (series-2.603/xdto-types.txt): the
	// envelope of SOAP and 64 of v8.1c.ru, 1720 names besides data/core's;
	// current-config gives none until its rule is shot.
	platform, names := 0, 0
	for space, list := range types {
		if strings.HasPrefix(space, "http://v8.1c.ru/") || strings.HasPrefix(space, "http://schemas.xmlsoap.org/") {
			platform++
			names += len(list)
		}
	}
	if platform != 66 || names != 1766 {
		t.Errorf("the platform's namespaces: %d of %d types, want 66 of 1766", platform, names)
	}
	if got := strings.Join(types["http://schemas.xmlsoap.org/soap/envelope/"], ","); got != "Body,detail,encodingStyle,Envelope,Fault,Header" {
		t.Errorf("the envelope of SOAP: %s", got)
	}
	if got := strings.Join(types["http://v8.1c.ru/8.1/xdto"], ","); got != "Form,Import,MemberTypes,Model,ObjectType,Package,Property,Type,ValueType,Variety,Whitespace" {
		t.Errorf("8.1/xdto: %s", got)
	}
	if got := types["http://v8.1c.ru/8.2/data/geo"]; len(got) != 31 || got[0] != "dp" || got[30] != "vi" {
		t.Errorf("8.2/data/geo, small letters among capitals: %d %q", len(got), got)
	}
	if got, ok := types["http://v8.1c.ru/8.1/data/enterprise/current-config"]; ok {
		t.Errorf("current-config gives types by no rule shot: %q", got)
	}
	if got := strings.Join(types["http://example.org/goods"], ","); got != "Адрес,Вид,Товар,цена" {
		t.Errorf("the package: %s", got)
	}
	if got, ok := types["http://example.org/empty"]; !ok || len(got) != 0 {
		t.Errorf("a package of no body: %q, %v", got, ok)
	}
}
